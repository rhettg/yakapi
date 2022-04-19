# Batteries Not Included

A [Yak Rover](https://github.com/The-Yak-Collective/yakrover) exploring flexible
components for a more extensible Rover.

Following my previous build, [A Stubborn Pursuit of a
Path](https://github.com/rhettg/stubborn) I was looking for a more extensible
build platform.

The mechanical system is built around
[Construx](https://www.youtube.com/watch?v=JJmKJyPviEA), a toy building system
from the 80s that this author has a lot of experience with and found in an old box.

Augmented with 3D printing, it has proven a very flexible build system.

## Components

* **Compute:** [Raspberry Pi Zero 2 W](https://www.raspberrypi.com/products/raspberry-pi-zero-2-w/)
* **Sensor/Neural Package**: [Oak-D-Lite](https://docs.luxonis.com/projects/hardware/en/latest/pages/DM9095.html)
* **Compute Battery:** Anker PowerCore Slim 10000mAh (Portable Charger)
* **Drive:** 2x N20, [Adafruit DC Motor + Stepper FeatherWing](https://learn.adafruit.com/adafruit-stepper-dc-motor-featherwing)
* **Drive Battery:** 6V 2000mAh NmH

### YakAPI

Implements a web api for interacting with the Rover. Based on the proposal in https://github.com/The-Yak-Collective/yakrover/pull/3

### Motor Adapter

Uses Circuit Python to provide a low-level interface into control motors.

### OakD

Uses depthai library to access OakD-lite capabilities
