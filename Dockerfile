FROM alpine:latest
RUN apk add bubblewrap
RUN apk add --repository "http://dl-cdn.alpinelinux.org/edge/community" proot
WORKDIR /app
COPY sandboxd .
RUN chmod +x sandboxd
EXPOSE 8080
CMD ["./sandboxd"]
