import asyncio
import aiohttp
import threading
import queue
import logging
import json

logger = logging.getLogger(__name__)


class Client:
    def __init__(self, base_url, max_retries=5, retry_delay=5):
        self.base_url = base_url
        self.max_retries = max_retries
        self.retry_delay = retry_delay
        self.loop = asyncio.new_event_loop()
        self.thread = threading.Thread(target=self._run_event_loop, daemon=True)
        self.thread.start()

    def _run_event_loop(self):
        asyncio.set_event_loop(self.loop)
        logger.debug("Starting event loop")
        self.loop.run_forever()

    def subscribe(self, stream_names):
        q = queue.Queue()
        asyncio.run_coroutine_threadsafe(
            self._async_subscribe(stream_names, q), self.loop
        )
        while True:
            yield q.get()

    async def _async_subscribe(self, stream_names, q):
        async with aiohttp.ClientSession() as session:
            tasks = [
                self._subscribe_with_retry(session, name, q) for name in stream_names
            ]
            await asyncio.gather(*tasks)

    async def _subscribe_with_retry(self, session, stream_name, q):
        retries = 0
        while True:
            try:
                await self._subscribe_stream(session, stream_name, q)
            except aiohttp.ClientError as e:
                logger.error(f"Error subscribing to stream {stream_name}: {e}")
                retries += 1
                if retries > self.max_retries:
                    logger.error(
                        f"Max retries reached for stream {stream_name}. Stopping."
                    )
                    return
                logger.info(
                    f"Retrying subscription to {stream_name} in {self.retry_delay} seconds..."
                )
                await asyncio.sleep(self.retry_delay)
            except asyncio.CancelledError:
                logger.info(f"Subscription to {stream_name} cancelled")
                return

    async def _subscribe_stream(self, session, stream_name, q):
        logger.debug(f"Subscribing to stream {stream_name}")
        async with session.get(
            f"{self.base_url}/v1/stream/{stream_name}", timeout=None
        ) as response:
            logger.debug(f"Subscribed to stream {stream_name}")
            buffer = b""
            async for data, end_of_http_chunk in response.content.iter_chunks():
                buffer += data
                if end_of_http_chunk:
                    logger.debug("received chunk")
                    event = json.loads(buffer.decode().strip())
                    q.put((stream_name, event))
                    buffer = b""

    def publish(self, stream_name, event):
        future = asyncio.run_coroutine_threadsafe(
            self._async_publish(stream_name, event), self.loop
        )
        return future.result()

    async def _async_publish(self, stream_name, event):
        async with aiohttp.ClientSession() as session:
            url = f"{self.base_url}/v1/stream/{stream_name}"
            async with session.post(url, json=event) as response:
                response.raise_for_status()
                return None
