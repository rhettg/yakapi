package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/rhettg/agent"
	"github.com/rhettg/agent/provider/openaichat"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/yaml.v3"
)

func errorf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}

func extractJSONBlock(text string) string {
	start := strings.Index(text, "```json")
	if start == -1 {
		return ""
	}

	start += len("```json")

	end := strings.Index(text[start:], "```")
	if end == -1 {
		return ""
	}
	// Adjust end index to account for the start offset
	end += start

	// Extract the JSON block without the markdown code block backticks
	return strings.TrimSpace(text[start:end])
}

//go:embed eyes.txt
var eyesSystemPrompt string

var prompt = `
Eyes, please describe the current scene in front of us, including the baby Yoda doll and any other significant objects or features.
`

var evalPrompt = `
You are evaluating the results of an AI vision system test. You will be provided
with test results that include a prompt (what was asked of the vision system),
reference description (what is actually in the image) as well as an AI generated
description of the scene.

Think step-by-step and explain your reasoning. Consider how well the AI
description describes the presence and location of the targetted object in the
description of the scene. Compare numeric values only of the target object to
determine how close they are to the expected results.  Ignore aspects of the ai
description that are not relevant to the test or details that are not available
in the reference description.

Finally, include in your reply a JSON object, formatted in a markdown json code block, in the following format:

{
	"TargetCheck": true, // true or false if the ai description and the reference description agree the target exists.
	"LeftRightCheck": true, // true or false if the ai description correctly identifies direction (left or right) of the target.
	"DegreeCheck": true, // true or false if the ai description correctly identifies the degree of the target to within 5 degrees.
	"DistanceCheck": true, // true or false if the ai description correctly identifies the distance of the doll to within 1 foot.
}

If the reference description indicates no target is expected and the ai correctly identifies the target is not in view, set all values to true.

Example step-by-step evaluation:

	The prompt indicates the target is a small squirrel. The ai description
	mentions seeing a squirrel about 3 feet away and about 10 degrees to the
	right. The reference description mentions the squirrel should be 20 feet away and 15 degrees to the right.

	TargetCheck: true because the target squirrel was correctly identified
	LeftRightCheck: true because the target was correctly identified as being to the right
	DegreeCheck: true because the target was identified as being 10 degrees which is within 5 degrees of the correct 15 degree value.
	DistanceCheck: false because the target was identified as being 3 feet away which is not within 1 foot of the correct 20 foot value.
`

var images = map[string]string{
	"iTerm2.BqxOKv.jpeg": "yoda doll on far left, -20 to -15 degrees. 4 feet away",
	"iTerm2.fCuCba.jpeg": "yoda doll on left, -13 degrees. 3 feet away",
	"iTerm2.MzUeEc.jpeg": "yoda doll on left, -11 degrees. 3 feet away",
	"iTerm2.OQALXt.jpeg": "yoda doll just on the left -5 degrees. close, about 24 inches awway",
	"iTerm2.QsEvrF.jpeg": "very close yoda doll, a few inches away. center to 5 degrees on the right",
	"iTerm2.Ypo7l0.jpeg": "no yoda doll, furniture",
	"iTerm2.sZOXSN.jpeg": "center yoda doll, 3 feet away",
	"iTerm2.z8lgU1.jpeg": "no yoda doll, whiteout, desk chair",
}

type evalResult struct {
	ImageName        string
	ImageDescription string

	Reply string

	TargetCheck    bool
	LeftRightCheck bool
	DegreeCheck    bool
	DistanceCheck  bool
	Score          float64
}

func (ev *evalResult) CalculateScore() {
	if !ev.TargetCheck {
		ev.Score = 0.0
		return
	}

	ev.Score += 0.50

	if ev.LeftRightCheck {
		ev.Score += 0.25
	}
	if ev.DegreeCheck {
		ev.Score += 0.15
	}
	if ev.DistanceCheck {
		ev.Score += 0.10
	}
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		errorf("OPENAI_API_KEY is not set")
	}

	client := openai.NewClient(apiKey)

	/*
		ev := evalResult{
			ImageName:        "iTerm2.BqxOKv.jpeg",
			ImageDescription: "yoda doll just on the left -5 degrees. close, about 24 inches away",
			Reply:            "I see a Baby Yoda doll standing directly ahead about 3 feet at 0-degrees. To the right of Baby Yoda, around 10-degrees, there is a tall object, possibly a cardboard box or a game packaging, that is aligned vertically and is touching the right edge at the bottom of the image. To the far right, at approximately 20-degrees, a partial view of what appears to be a black bag or case is visible, leaning against the vertical surface in the background. The floor appears to be a hard surface with a light color and some reflective qualities.",
		}

		err := evaluate(client, prompt, &ev)
		if err != nil {
			errorf("failed to evaluate test: %v\n", err)
		}

		ev.CalculateScore()
		fmt.Println("Score:", ev.Score)
		fmt.Printf("Target Check: %v\n", ev.TargetCheck)
		fmt.Printf("Left Right Check: %v\n", ev.LeftRightCheck)
		fmt.Printf("Degree Check: %v\n", ev.DegreeCheck)
		fmt.Printf("Distance Check: %v\n", ev.DistanceCheck)
		os.Exit(0)
	*/

	evals := make([]evalResult, 0)

	for name, description := range images {
		fmt.Println(name, description)

		ev := evalResult{
			ImageName:        name,
			ImageDescription: description,
		}

		err := eyes(client, &ev)
		if err != nil {
			errorf("failed to run test: %v\n", err)
		}

		err = evaluate(client, prompt, &ev)
		if err != nil {
			errorf("failed to evaluate test: %v\n", err)
		}

		ev.CalculateScore()
		fmt.Println("Score:", ev.Score)
		evals = append(evals, ev)
		fmt.Println()
	}

	// Marshall evals to yaml
	rf, err := os.Create("evals.yaml")
	if err != nil {
		errorf("failed to create evals.yaml: %v\n", err)
	}

	err = yaml.NewEncoder(rf).Encode(evals)
	if err != nil {
		errorf("failed to encode evals: %v\n", err)
	}
	rf.Close()
}

func evaluate(client *openai.Client, prompt string, ev *evalResult) error {
	ep := openaichat.New(client, "gpt-3.5-turbo",
		openaichat.WithMiddleware(openaichat.Logger(slog.Default())),
	)

	ea := agent.New(ep)
	ea.Add(agent.RoleSystem, evalPrompt)
	b := bytes.Buffer{}
	err := yaml.NewEncoder(&b).Encode(struct {
		Prompt               string
		AIDescription        string `yaml:"ai_description"`
		ReferenceDescription string `yaml:"reference_description"`
	}{
		Prompt:               prompt,
		AIDescription:        ev.Reply,
		ReferenceDescription: ev.ImageDescription,
	})
	if err != nil {
		return err
	}

	ea.Add(agent.RoleUser, b.String())
	evalReply, err := ea.Step(context.Background())
	if err != nil {
		return err
	}

	evalContent, err := evalReply.Content(context.Background())
	if err != nil {
		return err
	}

	fmt.Println(evalContent)

	jsonReply := extractJSONBlock(evalContent)
	if jsonReply == "" {
		return err
	}

	err = json.Unmarshal([]byte(jsonReply), ev)
	if err != nil {
		return err
	}

	return nil
}

func eyes(client *openai.Client, ev *evalResult) error {
	p := openaichat.New(client, "gpt-4-vision-preview",
		openaichat.WithMaxTokens(512),
		openaichat.WithMiddleware(openaichat.Logger(slog.Default())),
	)

	a := agent.New(p)
	a.Add(agent.RoleSystem, eyesSystemPrompt)

	imgData, err := os.ReadFile(fmt.Sprintf("test_images/%s", ev.ImageName))
	if err != nil {
		return err
	}
	msg := agent.NewImageMessage(agent.RoleUser, prompt, "eyes.jpg", imgData)
	a.AddMessage(msg)

	reply, err := a.Step(context.Background())
	if err != nil {
		return err
	}

	replyContent, err := reply.Content(context.Background())
	if err != nil {
		return err
	}

	ev.Reply = replyContent
	return nil
}
