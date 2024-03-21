FROM node:18-alpine3.15

WORKDIR /app

COPY . .

RUN yarn clean
RUN yarn install
RUN yarn build

ENTRYPOINT ["/app/run.sh"]
