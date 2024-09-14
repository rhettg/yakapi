# YakAPI Examples

This directory contains example scripts that demonstrate various functionalities of the YakAPI client. These examples showcase how to interact with the YakAPI server and utilize its features.

## Available Examples

1. `dump.py`: Subscribes to multiple streams and prints all received events.
2. `uptime.py`: Continuously sends telemetry data about the client's uptime to the server.
3. `ci.py`: Implements a "command injection" system, processing commands received from the server.

## Running the Examples

You can run these examples using any Python environment. One convenient way is to use `uv`, a fast Python package installer and resolver.

To run an example with `uv`:

      uv run script_name.py
   

Replace `script_name.py` with the name of the example you want to run.

For more information about `uv`, visit: https://github.com/astral-sh/uv

## Dependencies

The examples depend on the `yakapi` package, which is included in the `pyproject.toml` file. If you're not using `uv`, make sure to install the dependencies before running the examples.

## Exploring the Examples

Each example script contains comments explaining its functionality. Feel free to modify and experiment with these scripts to better understand how to use the YakAPI client in your own projects.

Happy coding with YakAPI!
