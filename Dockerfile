FROM alpine:3.20

WORKDIR /app

COPY servflow servflow

ENV SERVFLOW_PORT=8080
ENV SERVFLOW_LOGLEVEL=production
EXPOSE 8080
USER root
ENTRYPOINT ["./servflow"]
