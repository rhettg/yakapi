# YakAPI Client

YakAPI Client is a Python library for interacting with the YakAPI streaming service. It provides a simple and efficient way to subscribe to and publish events on YakAPI streams.

## Installation

To install YakAPI Client, you need Python 3.12 or later. You can install it using pip:

```
pip install yakapi
```

## Quick Start

Here's a simple example of how to use YakAPI Client:

```python
from yakapi import Client

# Initialize the client
client = Client("https://api.yakapi.com")

# Subscribe to a stream
for stream_name, event in client.subscribe(["my_stream"]):
    print(f"Received event from {stream_name}: {event}")

# Publish an event
client.publish("my_stream", {"message": "Hello, YakAPI!"})
```

## Features

- Easy-to-use API for subscribing to and publishing events
- Automatic reconnection and error handling
- Asynchronous operation using Python's asyncio

## Dependencies

- aiohttp>=3.10.5
- python-ulid>=2.7.0

## License

This project is licensed under the MIT License.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

If you have any questions or issues, please open an issue on our [GitHub repository](https://github.com/rhettg/yakapi).
