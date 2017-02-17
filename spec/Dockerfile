FROM node:alpine

RUN npm install -g raml2html

COPY . /contiv

WORKDIR /contiv

RUN raml2html -i contiv.raml -o contiv.html

ENTRYPOINT ["/bin/sh"]
