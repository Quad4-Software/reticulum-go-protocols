#!/usr/bin/env python3
"""JSON stdin/stdout harness for Go <-> rrcd (Python) RRC interop tests."""

from __future__ import annotations

import json
import shutil
import sys
import tempfile
import threading
import time
import traceback
from pathlib import Path
from typing import Any

import RNS
import cbor2

from rrcd.codec import decode as rrc_decode
from rrcd.codec import encode as rrc_encode
from rrcd.constants import (
    B_HELLO_CAPS,
    B_HELLO_NAME,
    B_HELLO_VER,
    B_LIMIT_MAX_MSG_BODY_BYTES,
    B_LIMIT_MAX_NICK_BYTES,
    B_LIMIT_MAX_ROOM_NAME_BYTES,
    B_LIMIT_MAX_ROOMS_PER_SESSION,
    B_LIMIT_RATE_LIMIT_MSGS_PER_MINUTE,
    B_WELCOME_CAPS,
    B_WELCOME_HUB,
    B_WELCOME_LIMITS,
    B_WELCOME_VER,
    CAP_ACTION,
    CAP_DIRECT_NOTICE,
    K_BODY,
    K_DST,
    K_ID,
    K_NICK,
    K_ROOM,
    K_SRC,
    K_T,
    K_TS,
    K_V,
    RRC_VERSION,
    T_HELLO,
    T_JOIN,
    T_JOINED,
    T_MSG,
    T_PART,
    T_WELCOME,
)
from rrcd.envelope import make_envelope, validate_envelope

RNS.loglevel = RNS.LOG_NONE


def _hex(b: bytes | None) -> str:
    if b is None:
        return ""
    return bytes(b).hex()


def _unhex(s: str) -> bytes:
    return bytes.fromhex(s)


def _jsonable(obj: Any) -> Any:
    if isinstance(obj, (bytes, bytearray)):
        return "hex:" + bytes(obj).hex()
    if isinstance(obj, dict):
        out: dict[str, Any] = {}
        for k, v in obj.items():
            key = str(int(k)) if isinstance(k, int) else str(k)
            out[key] = _jsonable(v)
        return out
    if isinstance(obj, (list, tuple)):
        return [_jsonable(x) for x in obj]
    return obj


def _from_jsonable(obj: Any) -> Any:
    if isinstance(obj, str) and obj.startswith("hex:"):
        return _unhex(obj[4:])
    if isinstance(obj, dict):
        out: dict[Any, Any] = {}
        for k, v in obj.items():
            try:
                key: Any = int(k)
            except (TypeError, ValueError):
                key = k
            out[key] = _from_jsonable(v)
        return out
    if isinstance(obj, list):
        return [_from_jsonable(x) for x in obj]
    return obj


def cmd_ping(_req: dict[str, Any]) -> dict[str, Any]:
    import rrcd

    ver = getattr(rrcd, "__version__", "unknown")
    return {"ok": True, "rrcd_version": ver, "rrc_version": RRC_VERSION}


def cmd_encode(req: dict[str, Any]) -> dict[str, Any]:
    msg_type = int(req["type"])
    src = _unhex(req["sender"])
    room = req.get("room")
    nick = req.get("nick")
    body = _from_jsonable(req.get("body")) if "body" in req else None
    dst = _unhex(req["destination"]) if req.get("destination") else None
    mid = _unhex(req["msg_id"]) if req.get("msg_id") else None
    ts = int(req["timestamp"]) if req.get("timestamp") is not None else None
    env = make_envelope(
        msg_type,
        src=src,
        dst=dst,
        room=room,
        body=body,
        nick=nick,
        mid=mid,
        ts=ts,
    )
    validate_envelope(env)
    packed = rrc_encode(env)
    return {
        "ok": True,
        "packed": _hex(packed),
        "envelope": _jsonable(env),
    }


def cmd_decode(req: dict[str, Any]) -> dict[str, Any]:
    packed = _unhex(req["packed"])
    env = rrc_decode(packed)
    validate_envelope(env)
    return {
        "ok": True,
        "envelope": _jsonable(env),
        "type": int(env[K_T]),
        "version": int(env[K_V]),
        "sender": _hex(env[K_SRC]),
        "msg_id": _hex(env[K_ID]),
        "timestamp": int(env[K_TS]),
        "room": env.get(K_ROOM),
        "nick": env.get(K_NICK),
        "destination": _hex(env.get(K_DST)) if env.get(K_DST) is not None else "",
        "body": _jsonable(env.get(K_BODY)),
    }


def cmd_validate(req: dict[str, Any]) -> dict[str, Any]:
    packed = _unhex(req["packed"])
    env = rrc_decode(packed)
    validate_envelope(env)
    return {"ok": True}


def _write_rns_config(configdir: Path, listen_port: int, forward_port: int) -> None:
    configdir.mkdir(parents=True, exist_ok=True)
    (configdir / "storage").mkdir(exist_ok=True)
    (configdir / "storage" / "identities").mkdir(exist_ok=True)
    cfg = f"""[reticulum]
  enable_transport = Yes
  share_instance = No
  shared_instance_port = 0
  instance_name = rrc-interop-{listen_port}

[logging]
  loglevel = 0

[interfaces]

  [[UDP Interface]]
    type = UDPInterface
    interface_enabled = True
    listen_ip = 127.0.0.1
    listen_port = {listen_port}
    forward_ip = 127.0.0.1
    forward_port = {forward_port}
"""
    (configdir / "config").write_text(cfg, encoding="utf-8")


def _link_send(link: RNS.Link, payload: bytes) -> None:
    RNS.Packet(link, payload).send()


def cmd_live_client(req: dict[str, Any]) -> dict[str, Any]:
    """Connect as an RRC client to a Go hub over UDP and exchange JOIN/MSG."""
    hub_hash = _unhex(req["hub_hash"])
    listen_port = int(req["listen_port"])
    forward_port = int(req["forward_port"])
    room = req.get("room", "#lobby")
    text = req.get("text", "hello from python")
    nick = req.get("nick", "py-client")
    timeout_s = float(req.get("timeout_s", 30))

    configdir = Path(tempfile.mkdtemp(prefix="rrc-interop-"))
    try:
        _write_rns_config(configdir, listen_port, forward_port)
        _reticulum = RNS.Reticulum(str(configdir))
        identity = RNS.Identity()

        deadline = time.time() + timeout_s
        while not RNS.Transport.has_path(hub_hash):
            if time.time() > deadline:
                return {"ok": False, "error": "path timeout to hub"}
            RNS.Transport.request_path(hub_hash)
            time.sleep(0.1)

        hub_id = RNS.Identity.recall(hub_hash)
        if hub_id is None:
            return {"ok": False, "error": "hub identity not recalled"}

        dest = RNS.Destination(
            hub_id,
            RNS.Destination.OUT,
            RNS.Destination.SINGLE,
            "rrc",
            "hub",
        )
        if dest.hash != hub_hash:
            return {
                "ok": False,
                "error": f"destination hash mismatch got={_hex(dest.hash)} want={_hex(hub_hash)}",
            }

        welcome_ev = threading.Event()
        joined_ev = threading.Event()
        welcome_body: dict[str, Any] = {}

        def on_packet(message: bytes, packet: Any) -> None:
            try:
                env = rrc_decode(message)
                validate_envelope(env)
            except Exception:
                return
            t = int(env[K_T])
            if t == T_WELCOME:
                welcome_body.clear()
                welcome_body.update(_jsonable(env.get(K_BODY) or {}))
                welcome_ev.set()
            elif t == T_JOINED:
                joined_ev.set()

        def on_established(link: RNS.Link) -> None:
            link.set_packet_callback(on_packet)
            link.identify(identity)
            hello = make_envelope(
                T_HELLO,
                src=identity.hash,
                nick=nick,
                body={
                    B_HELLO_NAME: "rrc-interoptest",
                    B_HELLO_VER: "0.1.0",
                    B_HELLO_CAPS: {},
                },
            )
            _link_send(link, rrc_encode(hello))

        link = RNS.Link(dest, established_callback=on_established)

        while link.status != RNS.Link.ACTIVE:
            if time.time() > deadline:
                return {"ok": False, "error": "link establish timeout"}
            time.sleep(0.05)

        if not welcome_ev.wait(timeout=max(1.0, deadline - time.time())):
            return {"ok": False, "error": "welcome timeout"}

        join = make_envelope(T_JOIN, src=identity.hash, room=room, nick=nick)
        _link_send(link, rrc_encode(join))
        if not joined_ev.wait(timeout=max(1.0, deadline - time.time())):
            return {"ok": False, "error": "joined timeout"}

        msg = make_envelope(
            T_MSG,
            src=identity.hash,
            room=room,
            body=text,
            nick=nick,
        )
        _link_send(link, rrc_encode(msg))
        time.sleep(0.3)

        part = make_envelope(T_PART, src=identity.hash, room=room, nick=nick)
        _link_send(link, rrc_encode(part))
        time.sleep(0.2)
        link.teardown()

        return {
            "ok": True,
            "client_hash": _hex(identity.hash),
            "welcome_body": welcome_body,
            "room": room,
            "text": text,
        }
    finally:
        try:
            shutil.rmtree(configdir, ignore_errors=True)
        except Exception:
            pass


def cmd_live_hub(req: dict[str, Any]) -> dict[str, Any]:
    """Run a minimal rrc.hub over UDP and wait for a client MSG."""
    listen_port = int(req["listen_port"])
    forward_port = int(req["forward_port"])
    ready_path = req["ready_path"]
    timeout_s = float(req.get("timeout_s", 40))
    hub_name = req.get("hub_name", "py-interop-hub")

    configdir = Path(tempfile.mkdtemp(prefix="rrc-interop-hub-"))
    try:
        _write_rns_config(configdir, listen_port, forward_port)
        _reticulum = RNS.Reticulum(str(configdir))
        identity = RNS.Identity()
        dest = RNS.Destination(
            identity,
            RNS.Destination.IN,
            RNS.Destination.SINGLE,
            "rrc",
            "hub",
        )

        got_text: dict[str, str] = {}
        msg_ev = threading.Event()

        def on_link(link: RNS.Link) -> None:
            def on_packet(message: bytes, packet: Any) -> None:
                try:
                    env = rrc_decode(message)
                    validate_envelope(env)
                except Exception:
                    return
                t = int(env[K_T])
                src = identity.hash
                if t == T_HELLO:
                    welcome = make_envelope(
                        T_WELCOME,
                        src=src,
                        body={
                            B_WELCOME_HUB: hub_name,
                            B_WELCOME_VER: "0.1.0",
                            B_WELCOME_CAPS: {
                                CAP_ACTION: True,
                                CAP_DIRECT_NOTICE: True,
                            },
                            B_WELCOME_LIMITS: {
                                B_LIMIT_MAX_NICK_BYTES: 32,
                                B_LIMIT_MAX_ROOM_NAME_BYTES: 64,
                                B_LIMIT_MAX_MSG_BODY_BYTES: 350,
                                B_LIMIT_MAX_ROOMS_PER_SESSION: 32,
                                B_LIMIT_RATE_LIMIT_MSGS_PER_MINUTE: 60,
                            },
                        },
                    )
                    _link_send(link, rrc_encode(welcome))
                elif t == T_JOIN:
                    room = env.get(K_ROOM) or "#lobby"
                    joined = make_envelope(T_JOINED, src=src, room=room)
                    _link_send(link, rrc_encode(joined))
                elif t == T_MSG:
                    body = env.get(K_BODY)
                    if isinstance(body, str):
                        got_text["text"] = body
                    elif isinstance(body, (bytes, bytearray)):
                        got_text["text"] = bytes(body).decode("utf-8", errors="replace")
                    else:
                        got_text["text"] = str(body)
                    msg_ev.set()

            link.set_packet_callback(on_packet)

        dest.set_link_established_callback(on_link)

        Path(ready_path).write_text(
            json.dumps(
                {
                    "hub_hash": _hex(dest.hash),
                    "public_key": _hex(identity.get_public_key()),
                }
            ),
            encoding="utf-8",
        )

        deadline = time.time() + timeout_s
        while time.time() < deadline:
            dest.announce()
            if msg_ev.wait(timeout=0.4):
                break

        if not msg_ev.is_set():
            return {"ok": False, "error": "timeout waiting for client MSG"}

        return {
            "ok": True,
            "hub_hash": _hex(dest.hash),
            "text": got_text.get("text", ""),
        }
    finally:
        try:
            shutil.rmtree(configdir, ignore_errors=True)
        except Exception:
            pass


def handle(req: dict[str, Any]) -> dict[str, Any]:
    cmd = req.get("cmd")
    if cmd == "ping":
        return cmd_ping(req)
    if cmd == "encode":
        return cmd_encode(req)
    if cmd == "decode":
        return cmd_decode(req)
    if cmd == "validate":
        return cmd_validate(req)
    if cmd == "live_client":
        return cmd_live_client(req)
    if cmd == "live_hub":
        return cmd_live_hub(req)
    return {"ok": False, "error": f"unknown cmd: {cmd!r}"}


def main() -> int:
    try:
        raw = sys.stdin.read()
        req = json.loads(raw) if raw.strip() else {}
        resp = handle(req)
    except Exception as exc:
        resp = {
            "ok": False,
            "error": str(exc),
            "trace": traceback.format_exc(),
        }
    sys.stdout.write(json.dumps(resp, separators=(",", ":")))
    sys.stdout.write("\n")
    return 0 if resp.get("ok") else 1


if __name__ == "__main__":
    raise SystemExit(main())
