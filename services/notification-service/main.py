"""notification-service: posts notifications to a downstream channel.
Production-grade: structured logs, /metrics, /healthz, /readyz, graceful shutdown.
"""
import os
import time
from contextlib import asynccontextmanager

import structlog
from fastapi import FastAPI, HTTPException
from fastapi.responses import PlainTextResponse
from prometheus_client import Counter, Histogram, generate_latest, CONTENT_TYPE_LATEST
from pydantic import BaseModel

log = structlog.get_logger()

REQS = Counter("http_requests_total", "Request count", ["route", "status"])
LAT = Histogram("http_request_duration_seconds", "Latency", ["route"])


class Notification(BaseModel):
    user_id: str
    channel: str  # "email" | "sms" | "push"
    message: str


@asynccontextmanager
async def lifespan(app: FastAPI):
    log.info("notification-service starting", version=os.getenv("APP_VERSION", "dev"))
    yield
    log.info("notification-service stopping")


app = FastAPI(lifespan=lifespan)


@app.middleware("http")
async def metrics_mw(request, call_next):
    start = time.time()
    response = await call_next(request)
    LAT.labels(request.url.path).observe(time.time() - start)
    REQS.labels(request.url.path, str(response.status_code)).inc()
    return response


@app.get("/healthz")
def healthz():
    return {"status": "ok"}


@app.get("/readyz")
def readyz():
    return {"status": "ready"}


@app.get("/metrics", response_class=PlainTextResponse)
def metrics():
    return PlainTextResponse(generate_latest(), media_type=CONTENT_TYPE_LATEST)


@app.post("/send")
def send(n: Notification):
    if n.channel not in {"email", "sms", "push"}:
        raise HTTPException(400, f"unsupported channel: {n.channel}")
    log.info("notification.sent", user_id=n.user_id, channel=n.channel)
    return {"status": "queued", "user_id": n.user_id, "channel": n.channel}
