#!/bin/bash
# Returns 99 if PALEO_RUNNER_CONFIG is set, 0 otherwise
if [ -n "$PALEO_RUNNER_CONFIG" ]; then
  echo '{"value": 99}'
else
  echo '{"value": 0}'
fi
