import requests
import json


class Client:
    def __init__(self, base_url):
        self.base_url = base_url

    def _stream_url(self, stream_name):
        return f"{self.base_url}/v1/stream/{stream_name}"

    def send(self, stream_name, command):
        response = requests.post(self._stream_url(stream_name), json=command)
        response.raise_for_status()
        return None

    def read_stream(self, stream_name):
        with requests.get(self._stream_url(stream_name), stream=True) as response:
            response.raise_for_status()
            for chunk in response.iter_content(chunk_size=1024, decode_unicode=True):
                yield json.loads(chunk.decode("utf-8").strip())
