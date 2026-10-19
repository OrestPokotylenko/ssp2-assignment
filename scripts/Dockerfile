FROM python:3.13-alpine

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /app

RUN addgroup -S app && adduser -S -G app app

COPY --chown=app:app message_generator.py messages.json ./

USER app

CMD ["python", "message_generator.py"]
