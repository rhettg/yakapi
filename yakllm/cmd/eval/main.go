package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
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
Eyes, please describe the current scene in front of us including any significant
objects or features. We're looking for a Baby Yoda doll which may or may not be
present.
`

var evalPrompt = `
You are evaluating the results of an AI vision system. You will be provided with
test results that include a prompt (what was asked of the vision system), and an
AI generated description of the scene.

Think step-by-step and explain your reasoning. Consider how well the AI
description describes the presence and location of the targetted object in the
description of the scene. Focus on numeric values only of the target object to
determine the expected valuesresults.  Ignore aspects of the ai description that
are not relevant to the test.

Finally, include in your reply a JSON object, formatted in a markdown json code block, in the following format:

` + "```json" + `
{
	"TargetExists": true,
	"TargetDirection": "left",
	"TargetDegree": -10,
	"DistanceInches": 28
}
` + "```\n" + `
The fields are:

* TargetExists: true if the ai description identifies the presence of the target.
* TargetDirection: which direction the ai description identifies the target to be in. Valid values: "left", "right", "center"
* TargetDegree: the ai description identifies the target to be at this degree. Negative values are to the left, positive values are to the right.
* DistanceInches: number of whole inches away from the target. Convert from units in the ai description.

Do not include fields without answers or leave them as empty strings or 0s.
`

var evalCases = []evalCase{
	{
		ImageName: "iTerm2.BqxOKv.jpeg",
		Description: description{
			TargetExists:    true,
			TargetDirection: "left",
			TargetDegree:    -18,
			DistanceInches:  4 * 12,
		},
	},
	{
		ImageName: "iTerm2.fCuCba.jpeg",
		Description: description{
			TargetExists:    true,
			TargetDirection: "left",
			TargetDegree:    -13,
			DistanceInches:  3 * 12,
		},
	},
	{
		ImageName: "iTerm2.MzUeEc.jpeg",
		Description: description{
			TargetExists:    true,
			TargetDirection: "left",
			TargetDegree:    -11,
			DistanceInches:  3 * 12,
		},
	},
	{
		ImageName: "iTerm2.OQALXt.jpeg",
		Description: description{
			TargetExists:    true,
			TargetDirection: "left",
			TargetDegree:    5,
			DistanceInches:  24,
		},
	},
	{
		ImageName: "iTerm2.Ypo7l0.jpeg",
		Description: description{
			TargetExists:    false,
			TargetDirection: "",
			TargetDegree:    0,
			DistanceInches:  0,
		},
	},
	{
		ImageName: "iTerm2.sZOXSN.jpeg",
		Description: description{
			TargetExists:    true,
			TargetDirection: "center",
			TargetDegree:    0,
			DistanceInches:  3 * 12,
		},
	},
	{
		ImageName: "iTerm2.z8lgU1.jpeg",
		Description: description{
			TargetExists:    false,
			TargetDirection: "",
			TargetDegree:    0,
			DistanceInches:  0,
		},
	},
}

type evalCase struct {
	ImageName string

	Description description
}

type description struct {
	TargetExists    bool
	TargetDirection string
	TargetDegree    float64
	DistanceInches  float64
}

type evalResult struct {
	ImageName string

	Reply string

	Description description
}

func calculateScore(ref description, ai description) float64 {
	if !ref.TargetExists && !ai.TargetExists {
		return 1.0
	}

	score := 0.0

	if ref.TargetExists == ai.TargetExists {
		score += 0.50
	}

	if ref.TargetDirection == ai.TargetDirection {
		score += 0.25
	}

	degreeError := math.Abs(float64(ref.TargetDegree - ai.TargetDegree))
	score += math.Max(0, 1-(degreeError/10.0)) * 0.15

	distanceError := math.Abs(float64(ref.DistanceInches - ai.DistanceInches))
	score += math.Max(0, 1-(distanceError/12.0)) * 0.10

	return score
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		errorf("OPENAI_API_KEY is not set")
	}

	client := openai.NewClient(apiKey)

	if false {
		ev := evalResult{
			ImageName: "iTerm2.BqxOKv.jpeg",
			Reply:     "I see a Baby Yoda doll to the left, at about -20 degrees, roughly 4 feet away. Directly ahead, there is a gap between some black panels or furniture at 0-degrees, around 6 feet away. To the right at approximately 15 degrees, there's a brown couch or similar furniture, also about 6 feet in the distance. The floor appears to be a smooth surface, suitable for rolling.",
		}

		var ec evalCase
		for _, c := range evalCases {
			if c.ImageName == ev.ImageName {
				ec = c
			}
		}

		err := evaluate(client, prompt, &ev)
		if err != nil {
			errorf("failed to evaluate test: %v\n", err)
		}

		fmt.Printf("Target: %v (%v)\n", ev.Description.TargetExists, ec.Description.TargetExists)
		fmt.Printf("Target Direction: %v (vs %v)\n", ev.Description.TargetDirection, ec.Description.TargetDirection)
		fmt.Printf("Target Degree: %v (vs %v)\n", ev.Description.TargetDegree, ec.Description.TargetDegree)
		fmt.Printf("Distance: %v (vs %v)\n", ev.Description.DistanceInches, ec.Description.DistanceInches)
		score := calculateScore(ec.Description, ev.Description)
		fmt.Println("Score:", score)
		os.Exit(0)
	}

	total := 0.0

	for _, ec := range evalCases {
		fmt.Println(ec.ImageName)

		ev := evalResult{
			ImageName: ec.ImageName,
		}

		fmt.Println("Asking eyes system...")
		err := eyes(client, &ev)
		if err != nil {
			errorf("failed to run test: %v\n", err)
		}
		fmt.Println(ev.Reply)

		err = evaluate(client, prompt, &ev)
		if err != nil {
			errorf("failed to evaluate test: %v\n", err)
		}

		score := calculateScore(ec.Description, ev.Description)
		fmt.Printf("Case: %v\n", ec.Description)
		fmt.Println("Score:", score)
		fmt.Println()
		total += score
	}
	fmt.Println("Overall Score: ", total/float64(len(evalCases)))

	// Marshall evals to yaml
	/*
		rf, err := os.Create("evals.yaml")
		if err != nil {
			errorf("failed to create evals.yaml: %v\n", err)
		}

		err = yaml.NewEncoder(rf).Encode(evals)
		if err != nil {
			errorf("failed to encode evals: %v\n", err)
		}
		rf.Close()
	*/
}

func evaluate(client *openai.Client, prompt string, ev *evalResult) error {
	ep := openaichat.New(client, "gpt-3.5-turbo",
		openaichat.WithMiddleware(openaichat.Logger(slog.Default())),
		openaichat.WithTemperature(0.0),
	)

	ea := agent.New(ep)
	ea.Add(agent.RoleSystem, evalPrompt)
	b := bytes.Buffer{}
	err := yaml.NewEncoder(&b).Encode(struct {
		Prompt        string
		AIDescription string `yaml:"ai_description"`
	}{
		Prompt:        prompt,
		AIDescription: ev.Reply,
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

	err = json.Unmarshal([]byte(jsonReply), &ev.Description)
	if err != nil {
		return err
	}

	return nil
}

func eyes(client *openai.Client, ev *evalResult) error {
	p := openaichat.New(client, "gpt-4-vision-preview",
		openaichat.WithMaxTokens(512),
		openaichat.WithMiddleware(openaichat.Logger(slog.Default())),
		openaichat.WithTemperature(0.0),
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
