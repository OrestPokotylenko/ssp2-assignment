FROM python:3.13-alpine

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir --requirement requirements.txt

RUN addgroup -S app && adduser -S -G app app

COPY --chown=app:app consumer.py .

USER app

CMD ["python", "consumer.py"]
