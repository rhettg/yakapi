# YakAPI Server

The YakAPI Server is meant to run on Rovers to provide an API for it. It is
extensible by allowing implementation specific software to participate in a
Rover software ecosystem.

## Usage

`yakapi` acts as a http server that presents a JSON API

### API

The JSON API is versioned, so start with `/v1` and you'll receive a nice hello:

```ShellSession
$ curl -s http://localhost:8080/v1 | jq .
{
  "name": "YakAPI (development)",
  "uptime": 1,
  "resources": [
    {
      "name": "metrics",
      "ref": "/metrics"
    },
    {
      "name": "eyes",
      "ref": "/eyes"
    },
    {
      "name": "eyes-api",
      "ref": "/v1/eyes/"
    },
    {
      "name": "stream",
      "ref": "/v1/stream/"
    },
    {
      "name": "project",
      "ref": "https://github.com/rhettg/yakapi"
    }
  ]
}
```

### Development

YakAPI lives by [scripts-to-rule-them-all](https://github.com/github/scripts-to-rule-them-all) rules.

But dependencies and setup are limited. Really you should be able to go out of the box with:

```ShellSession
$ script/server
2022-05-13T03:09:53.020Z        INFO    yakapi/main.go:218      starting        {"version": "1.0.0", "port": "8080"}
```

### Production

Sky is the limit, but for easy integration a `Dockerfile` is provided that is easily customized by environment variables.

```ShellSession

$ docker build -f Dockerfile -t yakapi:latest .
...
$ docker run --rm \
  -p 80:8080 \
  -e YAKAPI_NAME="My Rover" \
 yakapi:latest 
...
```

### Configuration

Configuration for the server is primarily through environment variables:

* `YAKAPI_PORT` [default `8080`] port for api server to listen on
* `YAKAPI_NAME` [default `YakBot`] name for rover 
* `YAKAPI_PROJECT_URL` [default `https://github.com/The-Yak-Collective/yakrover`] URL for more information

Other commands rely on:

* `YAKAPI_SERVER` [default `http://localhost:8080`] URL for api server (for non-server commands)


## Components

### Streams

The core interaction pattern for Yak API is to interact with streams of data.
Publishers stream data into Yak API. Subscribers retrieve that data.


Streams have a name and can be any content type.

#### Publishing

To publish data to a stream, send a `POST` request to the stream's URL such as
`/v1/streams/<stream_name>`. The request body should be the data to publish.
Optional `Content-Type` header can be used to specify the content type of the
data.

```ShellSession
$ curl -X POST -H "Content-Type: text/plain" \
  -d "hello world" \
  http://localhost:8080/v1/streams/test
```

The built-in CLI interface can also be used:

```ShellSession
$ echo "hello world" | yakapi pub test
```

#### Subscribing

To subscribe to a stream, send a `GET` request to the stream's URL. The response
will be a stream of the data. YakAPI makes use of Chunked Encoding to stream the
data. Each chunk will be an individual event. Most HTTP clients allow for
reading the stream in chunks, though sometimes care must be taken to ensure each event is received separately.

```ShellSession
$ curl --raw -s http://localhost:8080/v1/streams/test
"hello world"
```

The built-in CLI interface can also be used:

```ShellSession
$ yakapi sub test
hello world
```

### Eyes

The eyes component provides a mjpeg stream from the rover's camera.

### Telemetry

A stream named `telemetry` receives special handling. It is assumed to be of
content type `application/json` and exists as a JSON object mapping keys to
values.

By default, YakAPI publishes a single telemetry value `seconds_since_boot`.

### Metrics

Metrics are served in [prometheus](https://prometheus.io) format:

```ShellSession
$ curl -s http://localhost:8080/metrics
# HELP yakapi_processed_ops_total The total number of processed requests
# TYPE yakapi_processed_ops_total counter
yakapi_processed_ops_total 0
...

```

### YakGDS

YakAPI has built-in support for interacting with [YakGDS](https://github.com/rhettg/yakgds).

The `telemetry` stream will be posted as a "telemetry" note and displayed.

This behavior is deprecated and should exist as a separate component that simply
relies on the pub/sub mechanisms.

### Go API

A Go Client is available and can be seen in use via the
[`pub`](./internal/pub/pub.go) and [`sub`](./internal/sub/sub.go) commands.

```go
import github.com/rhettg/yakapi/client

c := client.NewClient("http://localhost:8080/")

c.Publish("test", []byte("hello world"), "text/plain")
```

```go
import github.com/rhettg/yakapi/client

c := client.NewClient("http://localhost:8080/")
eventChan, err := c.Subscribe("test")
if err != nil {
  return err
}

for event := range eventChan {
  fmt.Printf("%s: %s\n", event.StreamName, string(event.Data))
}
```

### Python API

Similiarly a Python client is available. Examples are available in the [examples](./examples) directory.

### ESP-IDF API

Code for interacting with the ESP-IDF exists and will be made available soon.
