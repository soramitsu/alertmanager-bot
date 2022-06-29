FROM alpine:3.16
ENV TEMPLATE_PATHS=/templates/default.tmpl

COPY ./default.tmpl /templates/default.tmpl
COPY ./alertmanager-bot /usr/bin/alertmanager-bot

RUN apk add --update ca-certificates && \
    chmod +x /usr/bin/alertmanager-bot && \
    addgroup -S appuser -g 15777 && adduser -DH appuser -u 15777 -G appuser

EXPOSE 8080
USER appuser

ENTRYPOINT ["/usr/bin/alertmanager-bot"]