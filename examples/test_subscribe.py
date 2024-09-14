import asyncio
import time
from aiohttp import web
from yakapi.yakapi import Client

# Mock server to simulate stream events
async def mock_server(request):
    response = web.StreamResponse(
        status=200,
        reason='OK',
        headers={'Content-Type': 'application/json'},
    )
    await response.prepare(request)
    for _ in range(3):
        await asyncio.sleep(1)
        await response.write(b'{"event": "test_event"}\n')
    return response

async def run_test():
    # Start mock server
    app = web.Application()
    app.router.add_get('/v1/stream/test', mock_server)
    runner = web.AppRunner(app)
    await runner.setup()
    site = web.TCPSite(runner, 'localhost', 8080)
    await site.start()

    # Initialize client
    client = Client('http://localhost:8080')

    # Test without timeout_event
    print("Testing without timeout_event:")
    events = client.subscribe(['test'])
    for _ in range(3):
        event = next(events)
        print(f"Received event: {event}")

    # Test with timeout_event
    print("\nTesting with timeout_event:")
    events_with_timeout = client.subscribe(['test'], timeout_event=0.5)
    start_time = time.time()
    for _ in range(5):
        event = next(events_with_timeout)
        print(f"Received event: {event}")
        if event[0] == 'timeout':
            print(f"Time since last event: {time.time() - start_time:.2f} seconds")
        start_time = time.time()

    # Cleanup
    await runner.cleanup()

if __name__ == '__main__':
    asyncio.run(run_test())
