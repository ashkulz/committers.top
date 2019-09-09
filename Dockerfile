FROM alpine:3.10

RUN apk --no-cache add curl git

COPY . /app

WORKDIR /app

ENTRYPOINT ["./generate_batch.sh"]
