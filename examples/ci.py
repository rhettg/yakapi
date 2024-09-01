import time
import logging

import yakapi


def motor_a(client, power):
    client.publish("motor_a", {"power": power})


def motor_b(client, power):
    client.publish("motor_b", {"power": power})


def apply_command(client, cmd, args):
    cmd = cmd.strip().lower()
    args = args.split(" ")

    if cmd == "quit":
        return None
    elif cmd in ("noop", "ping"):
        return 0.0
    elif cmd == "boom":
        raise Exception("boom")
    elif cmd == "lt":
        angle = args[0]
        motor_a(client, -0.8)
        motor_b(client, 0.8)
        return 1.32 * abs(float(angle) / 100)
    elif cmd == "rt":
        angle = args[0]
        motor_a(client, 0.8)
        motor_b(client, -0.8)
        return 1.32 * abs(float(angle)) / 100
    elif cmd == "fwd":
        duration = args[0]
        motor_a(client, 0.8)
        motor_b(client, 0.8)
        return float(duration) / 100
    elif cmd == "ffwd":
        duration = args[0]
        motor_a(client, 1.0)
        motor_b(client, 1.0)
        return float(duration) / 100
    elif cmd == "bck":
        duration = args[0]
        motor_a(client, -0.8)
        motor_b(client, -0.8)
        return float(duration) / 100
    else:
        raise Exception("Unknown command")


def main():
    client = yakapi.Client("http://localhost:8080")

    for _, event in client.subscribe(["ci"]):
        command = event.get("cmd")
        args = event.get("args", "")
        result = {"id": event["id"]}

        print(f"Processing {command} {args}... ", end="", flush=True)
        try:
            next_delay = apply_command(client, command, args)
        except Exception as e:
            print(f"error: {e}")
            result["error"] = str(e)
            client.publish("ci:result", result)
            continue

        if next_delay is None:
            print("quitting")
            break

        print("sleeping")
        time.sleep(next_delay)
        motor_a(client, 0)
        motor_b(client, 0)
        print("ok")

        result["result"] = "ok"
        client.publish("ci:result", result)


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    logging.debug("Starting")
    try:
        main()
    except KeyboardInterrupt:
        pass
    print("Bye!")
