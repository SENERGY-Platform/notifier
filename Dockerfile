FROM python:3.6-onbuild
LABEL org.opencontainers.image.source https://github.com/SENERGY-Platform/notifier

EXPOSE 5000

CMD [ "gunicorn", "--threads", "8", "-b", "0.0.0.0:5000", "--access-logfile", "-", "--keep-alive", "60", "main" ]
