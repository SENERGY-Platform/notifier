FROM python:3.9
LABEL org.opencontainers.image.source https://github.com/SENERGY-Platform/notifier

EXPOSE 5000
ADD . /opt/app
WORKDIR /opt/app
RUN pip install --no-cache-dir -r requirements.txt
USER 1000
CMD [ "uwsgi", "--http", ":5000", "--http-websockets", "--gevent", "100", "--wsgi", "main" ]
