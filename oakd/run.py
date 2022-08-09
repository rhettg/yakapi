#!/usr/local/bin/python3
import os
import time
from pathlib import Path
import cv2
import depthai as dai
import http.server

def setup_pipeline(pipeline):
    camRgb = pipeline.create(dai.node.ColorCamera)
    camRgb.setBoardSocket(dai.CameraBoardSocket.RGB)
    camRgb.setResolution(dai.ColorCameraProperties.SensorResolution.THE_1080_P)
    camRgb.setVideoSize(640, 480)
    camRgb.setFps(5)


    videoEnc = pipeline.create(dai.node.VideoEncoder)
    videoEnc.setDefaultProfilePreset(camRgb.getFps(), dai.VideoEncoderProperties.Profile.MJPEG)
    camRgb.video.link(videoEnc.input)

    xoutJpeg = pipeline.create(dai.node.XLinkOut)
    xoutJpeg.setStreamName("jpeg")
    videoEnc.bitstream.link(xoutJpeg.input)

boundary = '--boundarydonotcross'

def request_headers():
    return {
        'Cache-Control': 'no-store, no-cache, must-revalidate, pre-check=0, post-check=0, max-age=0',
        'Connection': 'close',
        'Content-Type': 'multipart/x-mixed-replace;boundary=%s' % boundary,
        'Expires': 'Mon, 3 Jan 2000 12:34:56 GMT',
        'Pragma': 'no-cache',
		'Access-Control-Allow-Origin': '*',
    }

def image_headers(size):
    return {
        'X-Timestamp': time.time(),
        'Content-Length': size,
        'Content-Type': 'image/jpeg',
    }

def build_handler(device):
    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            print("GET...")
            qJpg = device.getOutputQueue(name="jpeg", maxSize=30, blocking=True)

            self.send_response(200)

            # Response headers (multipart)
            for k, v in request_headers().items():
                self.send_header(k, v)

            while True:
                frame = qJpg.get()

                # Part boundary string
                self.end_headers()
                self.wfile.write(boundary.encode())
                self.end_headers()

                frame_data = frame.getData()

                # Part headers
                for k, v in image_headers(len(frame_data)).items():
                    self.send_header(k, v)
                self.end_headers()

                # Part binary
                self.wfile.write(frame_data)

        def log_message(self, format, *args):
            return

    return Handler

def main():
    print("booting...")
    try:
        print("configuring pipeline")
        pipeline = dai.Pipeline()
        setup_pipeline(pipeline)

        print("starting device")
        with dai.Device(pipeline) as device:
            httpd = http.server.HTTPServer(('', 8001), build_handler(device))

            print("starting server")
            httpd.serve_forever()

    except KeyboardInterrupt:
        print("done")
        return

if __name__ == "__main__":
    main()
