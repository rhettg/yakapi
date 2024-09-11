from yakapi import Client


def main():
    client = Client("http://localhost:8080")

    for stream, event in client.subscribe(
        ["ci", "ci:result", "motor_a", "motor_b", "telemetry"]
    ):
        print(stream)
        print(event)
        print()


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        pass
