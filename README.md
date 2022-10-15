# unicornsignage
A Go program for running on a raspberry pi that displays scrolling text on demand, and the weather when there is no text.

Designed for the [Unicorn Hat HD](https://shop.pimoroni.com/products/unicorn-hat-hd?variant=42496126730).

# Installation

First, install raspian onto your pi. Make sure the hostname is ```displaypi```. 
Then, create your own ```creds.json``` in the root of this repository. Format it like in [this example](/credexample.json). 
After that, run the Makefile with ```make firstdeploy```. 
Next, SSH into the rpi. In the rpi, run ```sudo nano /lib/systemd/system/display.service``` and paste the following text:

```
[Unit]
Description=Display device
After=network-online.target
Wants=network-online.target

[Install]
RequiredBy=multi-user.target

[Service]
Type=simple
User=root
WorkingDirectory=/home/pi
ExecStart=/home/pi/display
```
Finally, reload systemd ```sudo systemctl daemon-reload```, enable the service ```sudo systemctl enable display```, enable SPI and I2C support on the pi ```sudo raspi-config # Interface Options > SPI > Yes > Ok > Interface Options > I2C > Yes > Ok > Finish``` and restart the pi ```sudo reboot```.

Next time you make a change to the program, use ```make deploy``` instead of ```make firstdeploy``` to restart the service.

Enjoy!
