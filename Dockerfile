FROM python:3.6-onbuild

EXPOSE 5000

CMD [ "gunicorn", "-w", "8", "-b", "0.0.0.0:5000", "--access-logfile", "-", "main" ]