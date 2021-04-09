FROM python:3.9
LABEL org.opencontainers.image.source https://github.com/SENERGY-Platform/notifier

EXPOSE 5000
ADD . /opt/app
WORKDIR /opt/app
RUN pip install --no-cache-dir -r requirements.txt
CMD [ "gunicorn", "-k", "flask_sockets.worker", "-b", "0.0.0.0:5000", "--access-logfile", "-", "--keep-alive", "60", "main" ]
