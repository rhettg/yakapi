import time

import yakapi


def motor_a(client, power):
    client.send_command("motor_a", {"power": power})


def motor_b(client, power):
    client.send_command("motor_b", {"power": power})


def apply_command(client, cmd, args):
    cmd = cmd.strip().lower()
    args = args.split(" ")

    if cmd == "quit":
        return None
    elif cmd == "noop":
        return 0.0
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

    for event in client.read_stream("ci"):
        command = event.get("cmd")
        args = event.get("args", "")

        print(f"Processing {command} {args}... ", end="", flush=True)
        try:
            next_delay = apply_command(client, command, args)
        except Exception as e:
            client.send("ci_result", {"error": str(e)})
            continue

        print(f"sleeping")
        time.sleep(next_delay)
        motor_a(client, 0)
        motor_b(client, 0)
        print("done")

        client.send("ci_result", {"result": "ok"})


if __name__ == "__main__":
    main()
