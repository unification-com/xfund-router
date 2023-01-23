#!/usr/bin/make -f

abigen:
	npx truffle run abigen
	abigen --abi smart-contracrts/abigenBindings/abi/Router.abi --pkg ooo_router --out go-ooo/ooo_router/ooo_router.go

# Dev environment
dev-env:
	docker rm -f ooo_dev_env
	docker build -t ooo_dev_env -f docker/dev.Dockerfile .
	docker run --name ooo_dev_env -it -p 8545:8545 ooo_dev_env

.PHONY: abigen dev-env
