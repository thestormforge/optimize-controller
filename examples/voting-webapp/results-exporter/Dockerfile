FROM python:3.7-alpine

COPY . /

RUN \
  apk add --no-cache --virtual .build-deps gcc musl-dev && \
  pip3 install -r /requirements.txt && \
  apk --purge del .build-deps

env FLASK_APP /app.py

ENTRYPOINT ["flask", "run", "--host=0.0.0.0"]
