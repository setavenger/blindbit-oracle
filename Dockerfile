
FROM golang as buildstage

RUN mkdir /app
ADD . /app
WORKDIR /app


RUN go mod download
RUN env CGO_ENABLED=0 go build -o main ./src

FROM busybox
COPY --from=buildstage /app/main .

# CA certificates
COPY --from=buildstage /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
RUN mkdir "logs"
CMD ["./main"]
