import time

import yakapi


def send_telemetry(client, key, value):
    response = {key: str(value)}

    client.publish("telemetry", response)
    print(f"sent response: {response}")


def main():
    client = yakapi.Client("http://localhost:8080")

    startTime = time.time()
    while True:
        try:
            send_telemetry(client, "uptime", time.time() - startTime)
            time.sleep(1)
        except KeyboardInterrupt:
            print("Bye!")
            break


if __name__ == "__main__":
    main()
