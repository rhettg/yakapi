#!/usr/local/bin/python3
import os
import time
from pathlib import Path
import cv2
import depthai as dai

# Create pipeline
pipeline = dai.Pipeline()

# Define sources and outputs
camRgb = pipeline.create(dai.node.ColorCamera)
xoutRgb = pipeline.create(dai.node.XLinkOut)
imageManip = pipeline.create(dai.node.ImageManip)

xoutRgb.setStreamName("rgb")

# Properties
camRgb.setBoardSocket(dai.CameraBoardSocket.RGB)
camRgb.setResolution(dai.ColorCameraProperties.SensorResolution.THE_1080_P)
camRgb.setFps(1)

# Center crop/resize to 512x512
imageManip.initialConfig.setCropRect(0, 0, 1, 1)
imageManip.initialConfig.setResize(512, 512)
imageManip.initialConfig.setKeepAspectRatio(False)

# Linking
camRgb.video.link(imageManip.inputImage)
imageManip.out.link(xoutRgb.input)

# Connect to device and start pipeline
with dai.Device(pipeline) as device:
    print("booted")
    time.sleep(3)

    # Output queue will be used to get the rgb frames from the output defined above
    qRgb = device.getOutputQueue(name="rgb", maxSize=30, blocking=False)

    # Make sure the destination path is present before starting to store the examples
    dirName = os.getenv("CAPTURE_PATH")
    # Path(dirName).mkdir(parents=True, exist_ok=True)

    print("starting loop...")
    while True:
        print("looping...")
        inRgb = qRgb.tryGet()  # Non-blocking call, will return a new data that has arrived or None otherwise

        if inRgb is not None:
            print(f"writing frame to {dirName}")
            cv2.imwrite(f"{dirName}/capture.jpg", inRgb.getCvFrame())

        time.sleep(0.100)
