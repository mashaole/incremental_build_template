.PHONY: setup start start-golang start-nodejs stop status logs test

setup:
	./scripts/dev.sh setup

start:
	./scripts/dev.sh start

start-golang:
	./scripts/dev.sh start golang

start-nodejs:
	./scripts/dev.sh start nodejs

stop:
	./scripts/dev.sh stop

status:
	./scripts/dev.sh status

logs:
	./scripts/dev.sh logs

test:
	./scripts/dev.sh test
