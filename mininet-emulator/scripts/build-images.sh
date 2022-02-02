docker build --name=agent:latest --file=../agent/Dockerfile ../../dfaasagent
docker build --name=proxy:latest --file=../proxy/Dockerfile ../proxy