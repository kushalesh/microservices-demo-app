FROM python:3.12-slim AS build
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

FROM gcr.io/distroless/python3-debian12:nonroot
WORKDIR /app
COPY --from=build /root/.local /home/nonroot/.local
COPY main.py .
ENV PATH=/home/nonroot/.local/bin:$PATH
ENV PYTHONPATH=/home/nonroot/.local/lib/python3.12/site-packages
EXPOSE 5000
USER nonroot
CMD ["-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "5000"]
