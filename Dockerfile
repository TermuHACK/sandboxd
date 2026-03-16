FROM alpine:latest
RUN apk add --no-cache proot bubblewrap
WORKDIR /app
COPY sandboxd .
RUN chmod +x sandboxd
EXPOSE 8080
CMD ["./sandboxd"]
