FROM python:3.12-alpine

WORKDIR /app
COPY server.py /app/server.py
RUN addgroup -S sandbox && adduser -S sandbox -G sandbox
USER sandbox
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8080/health', timeout=2)"
CMD ["python", "-u", "/app/server.py"]
