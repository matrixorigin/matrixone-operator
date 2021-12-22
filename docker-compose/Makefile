.PHONY: matrixone production start
pro-start:
	docker-compose -f docker-compose.yml up -d

.PHONY: matrixone production stop 
pro-down:
	docker-compose -f docker-compose.yml down --remove-orphans

.PHONY: matrixone production clean
pro-clean:
	rm -rf log* && rm -rf store* && docker rmi matrixorigin/matrixone:latest
 
.PHONY: matrixone dev clone and build
dev-pre:
	git clone git@github.com:matrixorigin/matrixone.git -b ${BRANCH} && cp Dockerfile matrixone
	
.PHONY: matrixone dev image build
dev-build:
	cd matrixone && docker build . -t matrixone:dev

.PHONY: matrixone dev start
dev-start:
	docker-compose -f dev/docker-compose-dev.yml up -d

.PHONY: matrixone dev stop
dev-down:
	docker-compose -f dev/docker-compose-dev.yml down --remove-orphans

.PHONY: matrixone dev clean
dev-clean:
	rm -rf log-dev* && rm -rf store-dev* && rm -rf matrixone && docker rmi matrixone:dev
