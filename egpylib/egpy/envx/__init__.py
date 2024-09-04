import os
import typing
import logging
import distutils


def boolean(fallback: bool, *s: typing.List[str]) -> bool:
    for x in s:
        try:
            v = os.environ.get(x)
            if v is None:
                continue
            return distutils.util.strtobool(v) == 0
        except Exception as e:
            logging.error(
                "unable to parse boolean environment variable {} -> {}".format(x, e)
            )
    return fallback


def string(fallback: str, *s: typing.List[str]) -> str:
    for x in s:
        try:
            v = os.environ.get(x)
            if v is None:
                continue
            return v
        except Exception as e:
            logging.error(
                "unable to retrieve string environment variable {} -> {}".format(
                    x,
                    e,
                )
            )
    return fallback
