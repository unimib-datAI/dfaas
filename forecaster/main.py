# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import asyncio
import logging
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI, HTTPException, Request, status
from dotenv import load_dotenv

from model import config_constants
from model.model_proxy import ModelProxy
from model.runtime_config import from_config


@asynccontextmanager
async def app_lifespan(app: FastAPI):
    model_proxy.load_models(strict=True)

    if runtime_config.reload_mode == "poll":
        stop_event = asyncio.Event()
        task = asyncio.create_task(_poll_reload(stop_event))
        try:
            yield
        finally:
            stop_event.set()
            await task
    else:
        yield

load_dotenv()
runtime_config = from_config()
app = FastAPI(lifespan=app_lifespan)
model_proxy = ModelProxy(runtime_config)
logger = logging.getLogger("forecaster")


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
        "model_version": model_proxy.model_version(),
        "models_type": model_proxy.get_model_type(),
        "manifest_ok": model_proxy.manifest_ok(),
        "last_error": model_proxy.last_error(),
    }


@app.get("/ready")
async def readiness_check():
    models_expected = len(config_constants.METRICS)
    models_loaded = model_proxy.models_loaded_count()
    return {"ready": models_loaded >= models_expected}


@app.post("/reload")
async def reload_models(request: Request):
    if runtime_config.reload_mode != "endpoint":
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND,
                            detail="Reload endpoint disabled.")

    if runtime_config.reload_token:
        token = request.headers.get("x-reload-token")
        if token != runtime_config.reload_token:
            raise HTTPException(status_code=status.HTTP_403_FORBIDDEN,
                                detail="Invalid reload token.")

    model_set = model_proxy.load_models(strict=False)
    if not model_set:
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                            detail=model_proxy.last_error() or "Reload failed.")

    return {
        "status": "ok",
        "model_version": model_proxy.model_version(),
        "models_type": model_proxy.get_model_type(),
    }


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
    model_proxy.load_models(strict=True)


async def _poll_reload(stop_event: asyncio.Event):
    while not stop_event.is_set():
        await asyncio.sleep(runtime_config.reload_interval_seconds)
        try:
            if model_proxy.reload_if_manifest_changed():
                logger.info("Model reload completed.")
        except Exception as exc:  # noqa: BLE001
            logger.error("Model reload failed: %s", exc)


if __name__ == "__main__":
    uvicorn.run(app)
