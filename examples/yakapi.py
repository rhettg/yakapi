import requests
import json
import logging
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry

logger = logging.getLogger(__name__)


class Client:
    def __init__(self, base_url, user_agent=None):
        self.base_url = base_url
        self.session = requests.Session()
        retries = Retry(
            total=5, backoff_factor=0.1, status_forcelist=[500, 502, 503, 504]
        )
        self.session.mount("http://", HTTPAdapter(max_retries=retries))
        self.session.mount("https://", HTTPAdapter(max_retries=retries))

        if user_agent is None:
            user_agent = "YakapiClient/1.0"

        self.session.headers.update({"User-Agent": user_agent})

    def _stream_url(self, stream_name):
        return f"{self.base_url}/v1/stream/{stream_name}"

    def send(self, stream_name, command):
        logger.debug(f"Sending {command} to {stream_name}")
        try:
            response = self.session.post(
                self._stream_url(stream_name), json=command, timeout=10
            )
            response.raise_for_status()
            return None
        except requests.exceptions.RequestException as e:
            logger.error(f"Error sending command to {stream_name}: {e}")
            raise

    def read_stream(self, stream_name):
        logger.debug(f"Reading from {stream_name}")
        while True:
            with self.session.get(
                self._stream_url(stream_name), stream=True
            ) as response:
                response.raise_for_status()
                try:
                    for chunk in response.iter_content(
                        chunk_size=1024, decode_unicode=True
                    ):
                        if chunk:
                            try:
                                yield json.loads(chunk.strip())
                            except json.JSONDecodeError:
                                logger.error(f"Failed to parse JSON: {chunk}")
                except requests.exceptions.ChunkedEncodingError:
                    logger.warning("stream closed prematurely")
                    continue

    def close(self):
        self.session.close()
