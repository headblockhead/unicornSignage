build:
	GOOS=linux GOARCH=arm GOARM=5 go build cmd/main.go
deploy: build
	ssh pi@displaypi sudo systemctl stop display
	scp ./creds.json pi@displaypi:/home/pi/creds.json
	scp ./main "pi@displaypi:/home/pi/display"
	ssh pi@displaypi sudo systemctl start display