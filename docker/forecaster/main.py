import uvicorn
from fastapi import FastAPI, Request
from model.model_proxy import ModelProxy
from model import config_constants
from contextlib import asynccontextmanager
from dotenv import load_dotenv
import os


@asynccontextmanager
async def app_lifespan(app: FastAPI):
    load_models()
    yield

load_dotenv()
app = FastAPI(lifespan=app_lifespan)
model_proxy = ModelProxy(os.getenv('MODELS_TYPE'))


@app.get("/")
async def root():
    return "DFaaS Forecaster ready."


@app.get("/cpu_usage_node")
async def cpu_usage_node_prediction(request: Request):
    input_data_json = await request.json()
    return model_proxy.get_predictions(input_data_json,
                                       config_constants.CPU_USAGE_METRIC, True)


@app.get("/ram_usage_node")
async def ram_usage_node_prediction(request: Request):
    input_data_json = await request.json()
    return model_proxy.get_predictions(input_data_json,
                                       config_constants.RAM_USAGE_METRIC, True)


@app.get("/power_usage_node")
async def power_usage_node_prediction(request: Request):
    input_data_json = await request.json()
    return model_proxy.get_predictions(input_data_json,
                                       config_constants.POWER_USAGE_METRIC, True)


@app.get("/node_usage")
async def node_usage_prediction(request: Request):
    input_data_json = await request.json()
    return model_proxy.get_node_predictions(input_data_json)


def load_models():
    for metric in config_constants.METRICS:
        model_proxy.create_model(metric)


if __name__ == "__main__":
    uvicorn.run(app)

