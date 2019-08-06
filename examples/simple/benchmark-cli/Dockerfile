FROM gradle:4.10 as benchmark-cli

RUN git clone --depth 1 --branch v6.7.0 https://github.com/elastic/logstash.git \
  && gradle -p logstash/tools/benchmark-cli/ assemble

FROM docker.elastic.co/logstash/logstash-oss:6.7.0

COPY --from=benchmark-cli /home/gradle/logstash/tools/benchmark-cli/build/libs \
  /usr/share/logstash/tools/benchmark-cli/build/libs

ENV BATCH_SIZE=128 WORKERS=2 REPEAT_DATA=1 TESTCASE=baseline

ENTRYPOINT exec benchmark.sh --local-path $PWD --ls-batch-size $BATCH_SIZE --ls-workers $WORKERS --repeat-data $REPEAT_DATA --testcase $TESTCASE
