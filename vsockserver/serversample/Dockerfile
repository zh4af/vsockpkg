FROM amazonlinux as vsockserversample
RUN mkdir -p /app
COPY serversample /app/
WORKDIR /app
CMD ["/app/serversample", "-p", "3000", "-v", "8000"]
