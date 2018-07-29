FROM python:alpine

COPY hugo /usr/bin/hugo
RUN apk --no-cache add make && pip install sphinx sphinx-autobuild sphinx_rtd_theme

LABEL at.wharfr.wharfrat.shell="/bin/sh"
