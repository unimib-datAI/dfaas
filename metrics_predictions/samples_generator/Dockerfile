# set base image python:3.8-slim-buster
FROM python:3.8-slim-buster

# set working directory as app
WORKDIR /app

# Install prerequisites
RUN apt-get -y update; apt-get -y install curl

# copy requirements.txt file from local (source) to file structure of container (destination) 
COPY requirements.txt requirements.txt

# Install the requirements specified in file using RUN
RUN pip3 install -r requirements.txt

# copy all items in current local directory (source) to current container directory (destination)
COPY . .

ENV PYTHONUNBUFFERED=1

# command to run when image is executed inside a container
CMD [ "python3", "samples-generator.py", "50", "20s" ]