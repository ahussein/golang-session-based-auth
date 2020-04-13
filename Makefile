### --------------------------------------------------------------------------------------------------------------------
### Variables
### (https://www.gnu.org/software/make/manual/html_node/Using-Variables.html#Using-Variables)
### --------------------------------------------------------------------------------------------------------------------
BINARY_NAME?=session-based-signin
BUILD_SRC=./cmd
SRC_DIRS=internal cmd

BUILD_DIR?= build
GO_LINKER_FLAGS=-ldflags="-s -w"
GIT_HOOKS_DIR=.githooks

# colors
NO_COLOR=\033[0m
OK_COLOR=\033[32;01m
ERROR_COLOR=\033[31;01m
WARN_COLOR=\033[33;01m


### --------------------------------------------------------------------------------------------------------------------
### RULES
### (https://www.gnu.org/software/make/manual/html_node/Rule-Introduction.html#Rule-Introduction)
### --------------------------------------------------------------------------------------------------------------------

# Define phony targets (https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html)
.PHONY: all clean build test-unit dev-up api-doc code-style

all: clean build

build: build-api

build-api:
	@printf "$(OK_COLOR)==> Building API binary$(NO_COLOR)\n"
	@if [ ! -d ${BUILD_DIR} ] ; then mkdir -p ${BUILD_DIR} ; fi
	@GO111MODULE=on go build -o ${BUILD_DIR}/${BINARY_NAME}-api ${GO_LINKER_FLAGS} ${BUILD_SRC}/api


# Clean after build
clean:
	@printf "$(OK_COLOR)==> Cleaning project$(NO_COLOR)\n"
	@if [ -d ${BUILD_DIR} ] ; then rm -rf ${BUILD_DIR}/* ; fi

dev-up:
	@docker-compose up --remove-orphans -d

code-style:
	@golangci-lint run -v