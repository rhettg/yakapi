import time
import sys
import board
from adafruit_motorkit import MotorKit

kit = MotorKit(i2c=board.I2C())

def motor():
    kit.motor1.throttle = -0.99
    kit.motor2.throttle = -0.5
    #time.sleep(4)
    kit.motor1.throttle = 0.0
    kit.motor2.throttle = 0.0

def main():
    # process command line arguments
    for a in sys.argv[1:]:
        name, value = a.split(":")
        value = float(value)

        device = getattr(kit, name)
        device.throttle = value

if __name__ == "__main__":
    main()