package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type command struct {
	Value string
}

func (c command) Fields() []string {
	return strings.Fields(c.Value)
}

type doc struct {
	Commands []command
}

var motorAdapter = "echo"

func main() {
	if len(os.Args) > 1 {
		motorAdapter = os.Args[1]
	}

	s := bufio.NewScanner(os.Stdin)
	s.Split(bufio.ScanLines)

	d := doc{}

	for s.Scan() {
		l := strings.TrimSpace(s.Text())
		if l == "" {
			continue
		}

		d.Commands = append(d.Commands, command{Value: l})
	}

	for n, c := range d.Commands {
		err := Accept(context.Background(), c.Fields())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error at line %d: %s\n", n+1, err)
		}
	}
}

func Accept(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("empty command")
	}

	switch args[0] {
	case "fwd":
		return doFwd(ctx, args[1:])
	case "ffwd":
		return doFfwd(ctx, args[1:])
	case "bck":
		return doBck(ctx, args[1:])
	case "lt":
		return doLT(ctx, args[1:])
	case "rt":
		return doRT(ctx, args[1:])
	default:
		return errors.New("unknown command")
	}
}

func parseDurationArg(arg string) (time.Duration, error) {
	durationArg, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	duration := time.Duration(durationArg) * time.Millisecond * 10

	return duration, nil
}

func parseAngleArg(arg string) (time.Duration, error) {
	angleArg, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	duration := time.Duration(spinDurationSecs*(float64(angleArg)/90.0)) * time.Second
	return duration, nil
}

func doFwd(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arguments")
	}

	duration, err := parseDurationArg(args[0])
	if err != nil {
		return err
	}

	err = motorAndStop(ctx, -0.75, -0.75, duration)
	if err != nil {
		return err
	}

	return nil
}

func doFfwd(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arguments")
	}

	duration, err := parseDurationArg(args[0])
	if err != nil {
		return err
	}

	err = motorAndStop(ctx, -1.0, -1.0, duration)
	if err != nil {
		return err
	}

	return nil
}

func doBck(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arguments")
	}

	duration, err := parseDurationArg(args[0])
	if err != nil {
		return err
	}

	err = motorAndStop(ctx, 0.75, 0.75, duration)
	if err != nil {
		return err
	}

	return nil
}

const spinDurationSecs = 2.0

func doRT(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arguments")
	}

	duration, err := parseAngleArg(args[0])
	if err != nil {
		return err
	}

	err = motorAndStop(ctx, -0.75, 0.75, duration)
	if err != nil {
		return err
	}

	return nil
}

func doLT(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arguments")
	}

	duration, err := parseAngleArg(args[0])
	if err != nil {
		return err
	}

	err = motorAndStop(ctx, 0.75, -0.75, duration)
	if err != nil {
		return err
	}

	return nil
}

func execMotorAdapter(ctx context.Context, args []string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	cmd := exec.CommandContext(ctx, motorAdapter, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed running motor adapter: %w", err)
	}

	return nil
}

func motorAndStop(ctx context.Context, throttle1, throttle2 float64, d time.Duration) error {
	err := execMotorAdapter(ctx,
		[]string{
			fmt.Sprintf("motor1:%.2f", throttle1),
			fmt.Sprintf("motor2:%.2f", throttle2)})
	if err != nil {
		return err
	}

	fmt.Printf("sleeping for %.3fs\n", d.Seconds())
	time.Sleep(d)

	err = execMotorAdapter(ctx, []string{"motor1:0.0", "motor2:0.0"})
	if err != nil {
		return err
	}

	return nil
}
