# --- Frontend build ---
FROM node:22-bookworm-slim AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- Backend build ---
FROM golang:1.25-bookworm AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server_bin ./cmd/server

# --- Runtime (Go binary + static frontend + Python viz deps) ---
FROM python:3.12-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends fonts-dejavu-core libgomp1 \
    && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir \
    duckdb \
    matplotlib \
    seaborn \
    bokeh \
    plotly \
    pandas \
    numpy

WORKDIR /app
COPY --from=backend /server_bin ./server_bin
COPY --from=frontend /app/frontend/out ./frontend/out

RUN useradd --create-home --shell /usr/sbin/nologin appuser \
    && mkdir -p /app/uploads/plots \
    && chown -R appuser:appuser /app
USER appuser

ENV HOST=0.0.0.0 \
    PORT=3000 \
    STATIC_DIR=/app/frontend/out \
    UPLOAD_DIR=/app/uploads

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:3000/api/health', timeout=3)"

CMD ["./server_bin"]
