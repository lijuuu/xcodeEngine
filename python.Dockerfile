FROM python:3.9-slim
WORKDIR /app

CMD ["sh", "-c", "python3 -c \"$CODE\""]
