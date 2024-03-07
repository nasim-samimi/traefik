FROM alpine:3.19
RUN apk --no-cache add ca-certificates 

COPY ./dist/traefik /usr/local/bin/traefik
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /usr/local/bin/traefik
RUN chmod +x /entrypoint.sh
EXPOSE 80
ENTRYPOINT ["/entrypoint.sh"]
CMD ["traefik"]
