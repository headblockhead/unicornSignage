build:
	GOOS=linux GOARCH=arm GOARM=5 go build cmd/main.go
firstdeploy: build
	scp ./creds.json pi@displaypi.lan:/home/pi/creds.json
	scp ./main pi@displaypi.lan:/home/pi/display
deploy: build
	ssh pi@displaypi sudo systemctl stop display
	scp ./creds.json pi@displaypi.lan:/home/pi/creds.json
	scp ./main pi@displaypi.lan:/home/pi/display
	ssh pi@displaypi sudo systemctl start display
