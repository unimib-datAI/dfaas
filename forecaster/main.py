# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

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


@app.get("/health")
async def health_check():
    models_expected = len(config_constants.METRICS)
    models_loaded = model_proxy.models_loaded_count()
    return {
        "status": "ok",
        "ready": models_loaded >= models_expected,
        "models_loaded": models_loaded,
        "models_expected": models_expected,
    }


@app.get("/ready")
async def readiness_check():
    models_expected = len(config_constants.METRICS)
    models_loaded = model_proxy.models_loaded_count()
    return {"ready": models_loaded >= models_expected}


@app.get("/cpu_usage_node")
@app.post("/cpu_usage_node")
async def cpu_usage_node_prediction(request: Request):
    input_data_json = await request.json()
    return model_proxy.get_predictions(input_data_json,
                                       config_constants.CPU_USAGE_METRIC, True)


@app.get("/ram_usage_node")
@app.post("/ram_usage_node")
async def ram_usage_node_prediction(request: Request):
    input_data_json = await request.json()
    return model_proxy.get_predictions(input_data_json,
                                       config_constants.RAM_USAGE_METRIC, True)


@app.get("/power_usage_node")
@app.post("/power_usage_node")
async def power_usage_node_prediction(request: Request):
    input_data_json = await request.json()
    return model_proxy.get_predictions(input_data_json,
                                       config_constants.POWER_USAGE_METRIC, True)


@app.get("/node_usage")
@app.post("/node_usage")
async def node_usage_prediction(request: Request):
    input_data_json = await request.json()
    return model_proxy.get_node_predictions(input_data_json)


def load_models():
    if not model_proxy.get_model_type():
        raise RuntimeError("MODELS_TYPE is not set. Aborting model load.")
    for metric in config_constants.METRICS:
        model_proxy.create_model(metric)


if __name__ == "__main__":
    uvicorn.run(app)
