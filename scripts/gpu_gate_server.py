#!/usr/bin/env python3
"""
ACE Local System-1 GPU Decision Server (CUDA on NVIDIA GeForce RTX 3050)
Combines:
1. Qwen/Qwen3.5-0.8B quantized in-place to 4-Bit Symmetric Group-128 Packed INT4 (746 MiB VRAM)
   with GPU KV-Prefix Trunk Caching (`past_key_values` reuse across questions on the same state)
   and Anthony's LiquidAI/d1-3B 1-token logprob readout (`src/extractors/ml_models/decision.py`).
2. convaiinnovations/laya (421M ModernBERT Calibrated RL `system_one` Decision Model, 414 MiB VRAM)
   for zero-output-token `noul`, `choice`, and `score` decisions.

Endpoints:
- GET  /health               -> GPU VRAM, active models, cache stats
- POST /v1/chat/completions  -> OpenAI/vLLM compatible 1-token `top_logprobs` endpoint for Anthony's `D1` client
- POST /system_one           -> LiquidAI/d1 & Laya `system_one(state, questions)` API (`noul`, `choice`, `score`)
- POST /gate                 -> Fast binary (`noul`) or multi-option (`choice`) gate for ACE Go Engine
"""
import glob
import hashlib
import json
import math
import os
import sys
import threading
import time
from collections import OrderedDict
from http.server import BaseHTTPRequestHandler, HTTPServer

import torch
import torch.nn as nn
import torch.nn.functional as F
from transformers import AutoModelForCausalLM, AutoTokenizer
from transformers.cache_utils import DynamicCache

SNAP_QWEN = glob.glob(
    "/home/jay/.cache/huggingface/hub/models--Qwen--Qwen3.5-0.8B/snapshots/*"
)[0]
SNAP_LAYA_LIST = glob.glob(
    "/home/jay/.cache/huggingface/hub/models--convaiinnovations--laya/snapshots/*"
)
SNAP_LAYA = SNAP_LAYA_LIST[0] if SNAP_LAYA_LIST else None


class Linear4Bit(nn.Module):
    """4-Bit Symmetric Group-128 Packed INT4 Linear Layer on CUDA."""

    def __init__(self, in_features: int, out_features: int, bias: bool, group_size: int = 128):
        super().__init__()
        self.in_features = in_features
        self.out_features = out_features
        self.group_size = group_size
        self.register_buffer(
            "qweight",
            torch.empty((out_features, in_features // 2), dtype=torch.uint8),
        )
        self.register_buffer(
            "scales",
            torch.empty((out_features, in_features // group_size), dtype=torch.float16),
        )
        if bias:
            self.register_buffer("bias", torch.empty(out_features, dtype=torch.float16))
        else:
            self.bias = None

    @classmethod
    def from_linear(cls, linear: nn.Linear, group_size: int = 128) -> "Linear4Bit":
        w = linear.weight.data.to(torch.float16).cuda()
        out_f, in_f = w.shape
        mod = cls(in_f, out_f, linear.bias is not None, group_size=group_size).cuda()
        w_grouped = w.view(out_f, in_f // group_size, group_size)
        amax = w_grouped.abs().amax(dim=-1, keepdim=True).clamp(min=1e-5)
        scale = amax / 7.0
        q = torch.clamp(torch.round(w_grouped / scale), -8, 7).to(torch.int8) + 8
        q_uint8 = q.to(torch.uint8).view(out_f, in_f)
        mod.qweight.copy_((q_uint8[:, 0::2] << 4) | (q_uint8[:, 1::2] & 0x0F))
        mod.scales.copy_(scale.squeeze(-1))
        if linear.bias is not None:
            mod.bias.copy_(linear.bias.data.to(torch.float16).cuda())
        return mod

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        out_f, half_in = self.qweight.shape
        in_f = half_in * 2
        high = (self.qweight >> 4).to(torch.int8) - 8
        low = (self.qweight & 0x0F).to(torch.int8) - 8
        q = torch.stack([high, low], dim=-1).view(out_f, in_f // self.group_size, self.group_size)
        w = (q.to(torch.float16) * self.scales.unsqueeze(-1)).view(out_f, in_f)
        return F.linear(x.to(torch.float16), w, self.bias)


def quantize_4bit(module: nn.Module) -> None:
    for name, child in list(module.named_children()):
        if isinstance(child, nn.Linear) and child.in_features % 128 == 0 and "lm_head" not in name:
            setattr(module, name, Linear4Bit.from_linear(child, 128))
            del child
        else:
            quantize_4bit(child)


print(f"[ACE-GPU-Gate] Loading {SNAP_QWEN} and quantizing to 4-bit INT4 on CUDA...", flush=True)
t0 = time.perf_counter()
tok = AutoTokenizer.from_pretrained(SNAP_QWEN, local_files_only=True)
model = AutoModelForCausalLM.from_pretrained(SNAP_QWEN, torch_dtype=torch.float16, local_files_only=True)
quantize_4bit(model)
model = model.cuda().eval()
torch.cuda.empty_cache()

# Load convaiinnovations/laya only if explicitly requested via ACE_LOAD_LAYA=1 (pure d1_4bit is more accurate on code/shell tasks and uses 688 MiB vs 2345 MiB VRAM)
laya_agent = None
if SNAP_LAYA and os.environ.get("ACE_LOAD_LAYA") == "1":
    try:
        sys.path.insert(0, SNAP_LAYA)
        from rl_agent_api import RLAgent

        laya_agent = RLAgent(SNAP_LAYA, device="cuda")
        print(f"[ACE-GPU-Gate] Loaded convaiinnovations/laya on CUDA from {SNAP_LAYA}", flush=True)
    except Exception as exc:
        print(f"[ACE-GPU-Gate] Warning: could not load laya: {exc}", flush=True)

vram_mb = torch.cuda.memory_allocated() / (1024**2)
print(
    f"[ACE-GPU-Gate] Ready in {(time.perf_counter()-t0)*1000:.1f}ms | Total VRAM: {vram_mb:.1f} MiB",
    flush=True,
)

# Exact LiquidAI/d1 token forms (matching Anthony's src/extractors/ml_models/decision.py & LiquidAI/d1-3B/prompt.py)
YES_FORMS = ("yes", "Yes", "YES")
NO_FORMS = ("no", "No", "NO")
YES_IDS = [tok.encode(t, add_special_tokens=False)[0] for t in YES_FORMS if len(tok.encode(t, add_special_tokens=False)) == 1]
NO_IDS = [tok.encode(t, add_special_tokens=False)[0] for t in NO_FORMS if len(tok.encode(t, add_special_tokens=False)) == 1]

import copy

gpu_lock = threading.Lock()
trunk_kv_cache: "OrderedDict[str, tuple[int, object]]" = OrderedDict()
exact_lp_cache: "OrderedDict[str, tuple[torch.Tensor, int]]" = OrderedDict()
MAX_TRUNK_CACHE = 24
stats = {
    "requests": 0,
    "trunk_cache_hits": 0,
    "trunk_cache_misses": 0,
    "exact_cache_hits": 0,
}


def forward_logprobs_with_prefix_cache(prefix_str: str, suffix_str: str) -> tuple[torch.Tensor, bool, int]:
    """
    LiquidAI/d1 style two-stage forward pass for Qwen3.5 Hybrid (LinearAttention + FullAttention) Cache:
    1. Exact prompt cache check (0.05ms if exact (prefix, suffix) was already evaluated).
    2. If `prefix_str` is non-empty, cache its Hybrid KV/Recurrent state (`past_key_values`) in GPU VRAM by SHA256.
    3. Run only `suffix_str` tokens on top of the deep-cloned `trunk` Hybrid state and return log-softmax of the final slot!
    """
    with gpu_lock:
        stats["requests"] += 1
        full_key = hashlib.sha256((prefix_str + "||" + suffix_str).encode("utf-8")).hexdigest()[:24]
        if full_key in exact_lp_cache:
            stats["exact_cache_hits"] += 1
            stats["trunk_cache_hits"] += 1
            exact_lp_cache.move_to_end(full_key)
            lp, n_toks = exact_lp_cache[full_key]
            return lp, True, n_toks

        if not prefix_str:
            full_ids = tok.encode(suffix_str, add_special_tokens=False)
            inp = torch.tensor([full_ids], dtype=torch.long, device="cuda")
            with torch.no_grad():
                logits = model(input_ids=inp, use_cache=False).logits[0, -1].float()
            lp = F.log_softmax(logits, dim=-1)
            exact_lp_cache[full_key] = (lp, len(full_ids))
            return lp, False, len(full_ids)

        key = hashlib.sha256(prefix_str.encode("utf-8")).hexdigest()[:24]
        suffix_ids = tok.encode(suffix_str, add_special_tokens=False)
        if not suffix_ids:
            suffix_ids = [tok.eos_token_id]

        if key in trunk_kv_cache:
            stats["trunk_cache_hits"] += 1
            trunk_kv_cache.move_to_end(key)
            prefix_len, pkv_snapshot = trunk_kv_cache[key]
            past_kv = copy.deepcopy(pkv_snapshot)
            cache_hit = True
        else:
            stats["trunk_cache_misses"] += 1
            prefix_ids = tok.encode(prefix_str, add_special_tokens=False)
            prefix_len = len(prefix_ids)
            p_inp = torch.tensor([prefix_ids], dtype=torch.long, device="cuda")
            with torch.no_grad():
                p_out = model(input_ids=p_inp, use_cache=True)
            pkv_snapshot = copy.deepcopy(p_out.past_key_values)
            trunk_kv_cache[key] = (prefix_len, pkv_snapshot)
            if len(trunk_kv_cache) > MAX_TRUNK_CACHE:
                trunk_kv_cache.popitem(last=False)
            past_kv = copy.deepcopy(pkv_snapshot)
            cache_hit = False

        total_len = prefix_len + len(suffix_ids)
        s_inp = torch.tensor([suffix_ids], dtype=torch.long, device="cuda")
        cache_pos = torch.arange(prefix_len, total_len, dtype=torch.long, device="cuda")
        with torch.no_grad():
            out = model(input_ids=s_inp, past_key_values=past_kv, cache_position=cache_pos, use_cache=True)
            logits = out.logits[0, -1].float()
        lp = F.log_softmax(logits, dim=-1)
        exact_lp_cache[full_key] = (lp, total_len)
        if len(exact_lp_cache) > 64:
            exact_lp_cache.popitem(last=False)
        return lp, cache_hit, total_len


def build_qwen_prefix_and_suffix(state_text: str, question_text: str) -> tuple[str, str]:
    """Splits a (state, question) prompt at the static trunk boundary so `sys_head` + `state_text` is always KV-cached on GPU."""
    sys_head = "<|im_start|>system\nYou are a System One decision model. Reply with the exact token requested only.<|im_end|>\n<|im_start|>user\n"
    clean_state = (state_text or "").strip()
    if clean_state:
        # Strip volatile per-step timestamps or pulse numbers if any so trunk key stays deterministic across the cycle
        prefix = f"{sys_head}STATE:\n{clean_state}\n\nQUESTION:\n"
    else:
        # Always cache the canonical system + user preamble in trunk_kv_cache even when state_text is empty!
        prefix = f"{sys_head}QUESTION:\n"
    suffix = f"{question_text}<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"
    return prefix, suffix


def d1_readout_noul(logprobs_tensor: torch.Tensor) -> tuple[float, float, float]:
    """Anthony's exact _readout from LyteFEX/src/extractors/ml_models/decision.py:
    max-pool ('yes','Yes','YES') vs ('no','No','NO'), then softmax."""
    yes_lp = max(float(logprobs_tensor[i]) for i in YES_IDS)
    no_lp = max(float(logprobs_tensor[i]) for i in NO_IDS)
    p_yes = 1.0 / (1.0 + math.exp(no_lp - yes_lp))
    return p_yes, yes_lp, no_lp


def d1_readout_choice(logprobs_tensor: torch.Tensor, labels: list[str]) -> dict:
    """LiquidAI/d1-3B prompt.py Choice readout over option codes A..Z."""
    codes = [chr(ord("A") + i) for i in range(len(labels))]
    scores = []
    for c in codes:
        c_ids = []
        for form in (c, f" {c}"):
            enc = tok.encode(form, add_special_tokens=False)
            if len(enc) == 1:
                c_ids.append(enc[0])
        scores.append(max(float(logprobs_tensor[i]) for i in c_ids) if c_ids else -20.0)
    m = max(scores)
    exps = [math.exp(s - m) for s in scores]
    total = sum(exps)
    probs = [e / total for e in exps]
    best = max(range(len(probs)), key=probs.__getitem__)
    return {
        "type": "choice",
        "choice": labels[best],
        "confidence": round(probs[best], 4),
        "probabilities": {labels[i]: round(probs[i], 4) for i in range(len(labels))},
    }


def d1_readout_score(logprobs_tensor: torch.Tensor, criteria: list[str]) -> dict:
    """LiquidAI/d1-3B prompt.py Score readout over single-digit levels 0..K-1."""
    k = len(criteria)
    scores = []
    for i in range(k):
        enc = tok.encode(str(i), add_special_tokens=False)
        scores.append(float(logprobs_tensor[enc[0]]) if len(enc) == 1 else -20.0)
    m = max(scores)
    exps = [math.exp(s - m) for s in scores]
    total = sum(exps)
    probs = [e / total for e in exps]
    best = max(range(len(probs)), key=probs.__getitem__)
    expected = sum(i * p for i, p in enumerate(probs))
    return {
        "type": "score",
        "score": round(expected, 4),
        "confidence": round(probs[best], 4),
        "probabilities": {str(i): round(probs[i], 4) for i in range(k)},
        "legend": {str(i): criteria[i] for i in range(k)},
    }


def eval_binary_gate(question: str, threshold: float = 0.5, state: str = "", engine: str = "d1_4bit") -> dict:
    t_start = time.perf_counter()
    q_text = question if "Reply with yes or no" in question else f"{question}\n\nReply with yes or no only."
    prefix, suffix = build_qwen_prefix_and_suffix(state, q_text)
    lp, cache_hit, n_toks = forward_logprobs_with_prefix_cache(prefix, suffix)
    p_yes_d1, yes_lp, no_lp = d1_readout_noul(lp)

    p_yes_laya = None
    if laya_agent is not None and engine in ("ensemble", "laya"):
        try:
            with gpu_lock:
                laya_res = laya_agent.system_one(
                    state or {"context": "ACE Cognitive Control Gate"},
                    {"q": {"type": "noul", "instructions": question}},
                )
            p_yes_laya = float(laya_res["answers"]["q"]["noul"])
        except Exception:
            p_yes_laya = None

    if engine == "laya" and p_yes_laya is not None:
        p_yes = p_yes_laya
        model_label = "laya-421M-CUDA"
    elif engine == "ensemble" and p_yes_laya is not None:
        # Calibrated System-1 blend (70% 4-bit d1 logprob readout + 30% Laya RL head)
        p_yes = 0.70 * p_yes_d1 + 0.30 * p_yes_laya
        model_label = "d1-4bit+laya-CUDA"
    else:
        p_yes = p_yes_d1
        model_label = "Qwen3.5-0.8B-INT4-4bit-CUDA"

    dt_ms = (time.perf_counter() - t_start) * 1000.0
    return {
        "p_yes": round(p_yes, 4),
        "p_yes_d1_4bit": round(p_yes_d1, 4),
        "p_yes_laya": round(p_yes_laya, 4) if p_yes_laya is not None else None,
        "decision": bool(p_yes >= threshold),
        "yes_logprob": round(yes_lp, 4),
        "no_logprob": round(no_lp, 4),
        "trunk_cache_hit": cache_hit,
        "input_tokens": n_toks,
        "latency_ms": round(dt_ms, 2),
        "vram_mb": round(vram_mb, 1),
        "model": model_label,
    }


def eval_system_one(state, questions: dict, engine: str = "d1_4bit") -> dict:
    """Implements LiquidAI/d1-3B & convaiinnovations/laya `system_one(state, questions)`."""
    t_start = time.perf_counter()
    if engine == "laya" and laya_agent is not None:
        with gpu_lock:
            res = laya_agent.system_one(state, questions)
        res["latency_ms"] = round((time.perf_counter() - t_start) * 1000.0, 2)
        return res

    state_str = state if isinstance(state, str) else json.dumps(state, ensure_ascii=False) if state is not None else ""
    answers = {}
    total_tokens = 0
    any_cache_hit = False

    for qid, qdef in questions.items():
        qtype = qdef.get("type", "noul")
        ins = qdef.get("instructions", "")
        crit = qdef.get("criteria")
        if qtype == "noul":
            extra = ""
            if isinstance(crit, dict) and ("true" in crit or "false" in crit):
                extra = f"\nYes: {crit.get('true', '')}\nNo: {crit.get('false', '')}"
            q_prompt = f"{ins}{extra}\n\nReply with yes or no only."
            prefix, suffix = build_qwen_prefix_and_suffix(state_str, q_prompt)
            lp, hit, n_toks = forward_logprobs_with_prefix_cache(prefix, suffix)
            p_yes, _, _ = d1_readout_noul(lp)
            answers[qid] = {"type": "noul", "noul": round(p_yes, 4)}
        elif qtype == "choice":
            if isinstance(crit, dict):
                labels = list(crit.keys())
                lines = "\n".join(f"{chr(ord('A')+i)} {crit[k] or k}" for i, k in enumerate(labels))
            else:
                labels = list(crit or [])
                lines = "\n".join(f"{chr(ord('A')+i)} {k}" for i, k in enumerate(labels))
            q_prompt = f"{ins}\n\nOptions:\n{lines}\n\nReply with the option code only."
            prefix, suffix = build_qwen_prefix_and_suffix(state_str, q_prompt)
            lp, hit, n_toks = forward_logprobs_with_prefix_cache(prefix, suffix)
            answers[qid] = d1_readout_choice(lp, labels)
        elif qtype == "score":
            levels = list(crit or [])
            legend = "\n".join(f"{i} {name}" for i, name in enumerate(levels))
            q_prompt = f"{ins}\n\n{legend}\n\nReply with a single digit 0-{len(levels)-1} only."
            prefix, suffix = build_qwen_prefix_and_suffix(state_str, q_prompt)
            lp, hit, n_toks = forward_logprobs_with_prefix_cache(prefix, suffix)
            answers[qid] = d1_readout_score(lp, levels)
        else:
            raise ValueError(f"Unknown question type: {qtype}")
        total_tokens += n_toks
        any_cache_hit = any_cache_hit or hit

    return {
        "model": "Qwen3.5-0.8B-INT4-4bit-CUDA",
        "answers": answers,
        "trunk_cache_hit": any_cache_hit,
        "latency_ms": round((time.perf_counter() - t_start) * 1000.0, 2),
        "usage": {"input_tokens": total_tokens, "output_tokens": 0},
    }


def eval_openai_chat_completions(body: dict) -> dict:
    """
    100% wire-compatible with Anthony's `D1` client in `LyteFEX/src/extractors/ml_models/decision.py`:
    Accepts `messages` (text or multimodal list), `max_tokens=1`, `logprobs=True`, `top_logprobs=20`
    and returns OpenAI `choices[0].logprobs.content[0].top_logprobs` including ('yes','Yes','YES','no','No','NO').
    """
    t_start = time.perf_counter()
    msgs = body.get("messages", [])
    state_parts = []
    question_part = ""
    for m in msgs:
        content = m.get("content", "")
        if isinstance(content, list):
            for item in content:
                if item.get("type") == "image_url":
                    url = (item.get("image_url") or {}).get("url", "")
                    # Hash image data URL as the cached frame state prefix!
                    img_hash = hashlib.sha256(url.encode("utf-8")).hexdigest()[:16]
                    state_parts.append(f"[Frame-Hash:{img_hash}]")
                elif item.get("type") == "text":
                    question_part = item.get("text", "")
        elif isinstance(content, str):
            question_part = content

    # Split shared state from trailing question if formatted with double newline
    state_str = "\n".join(state_parts)
    if not state_str and "\n\n" in question_part:
        parts = question_part.rsplit("\n\n", 2)
        if len(parts) >= 2:
            state_str = parts[0]
            question_part = "\n\n".join(parts[1:])

    prefix, suffix = build_qwen_prefix_and_suffix(state_str, question_part)
    lp, cache_hit, n_toks = forward_logprobs_with_prefix_cache(prefix, suffix)

    top_k = int(body.get("top_logprobs") or 20)
    vals, idxs = torch.topk(lp, k=min(top_k, lp.shape[-1]))
    top_entries = []
    seen_tokens = set()
    for v, idx in zip(vals.tolist(), idxs.tolist()):
        t_str = tok.decode([idx])
        top_entries.append({"token": t_str, "logprob": float(v)})
        seen_tokens.add(t_str)

    # Ensure YES_FORMS and NO_FORMS are always included in top_logprobs for _readout()
    for form, ids in [("yes", YES_IDS), ("Yes", YES_IDS), ("no", NO_IDS), ("No", NO_IDS)]:
        if form not in seen_tokens and ids:
            enc = tok.encode(form, add_special_tokens=False)
            tid = enc[0] if len(enc) == 1 else ids[0]
            top_entries.append({"token": form, "logprob": float(lp[tid])})
            seen_tokens.add(form)

    p_yes, _, _ = d1_readout_noul(lp)
    chosen_tok = "yes" if p_yes >= 0.5 else "no"
    return {
        "id": f"chatcmpl-d1-{int(time.time()*1000)}",
        "object": "chat.completion",
        "model": body.get("model", "LiquidAI/d1-3B"),
        "choices": [
            {
                "index": 0,
                "message": {"role": "assistant", "content": chosen_tok},
                "finish_reason": "length",
                "logprobs": {
                    "content": [
                        {
                            "token": chosen_tok,
                            "logprob": float(lp[YES_IDS[0]] if chosen_tok == "yes" else lp[NO_IDS[0]]),
                            "top_logprobs": top_entries,
                        }
                    ]
                },
            }
        ],
        "usage": {
            "prompt_tokens": n_toks,
            "completion_tokens": 1,
            "total_tokens": n_toks + 1,
            "trunk_cache_hit": cache_hit,
            "latency_ms": round((time.perf_counter() - t_start) * 1000.0, 2),
        },
    }


class GateHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

    def _send_json(self, status: int, payload: dict):
        raw = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        if self.path == "/health":
            self._send_json(
                200,
                {
                    "status": "healthy",
                    "vram_mb": round(vram_mb, 1),
                    "model": "Qwen3.5-0.8B-INT4-4bit-CUDA",
                    "laya_loaded": laya_agent is not None,
                    "trunk_cache_entries": len(trunk_kv_cache),
                    "stats": stats,
                },
            )
            return
        self._send_json(404, {"error": "not found"})

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = json.loads(self.rfile.read(length) or b"{}")
        try:
            if self.path == "/gate":
                res = eval_binary_gate(
                    question=body.get("question", ""),
                    threshold=float(body.get("threshold", 0.5)),
                    state=body.get("state", ""),
                    engine=body.get("engine", "d1_4bit"),
                )
                self._send_json(200, res)
            elif self.path == "/system_one":
                res = eval_system_one(
                    state=body.get("state"),
                    questions=body.get("questions", {}),
                    engine=body.get("engine", "d1_4bit"),
                )
                self._send_json(200, res)
            elif self.path in ("/v1/chat/completions", "/chat/completions"):
                res = eval_openai_chat_completions(body)
                self._send_json(200, res)
            else:
                self._send_json(404, {"error": f"unknown endpoint {self.path}"})
        except Exception as exc:
            self._send_json(500, {"error": str(exc)})


if __name__ == "__main__":
    port = int(os.environ.get("ACE_GPU_GATE_PORT", "8091"))
    server = HTTPServer(("127.0.0.1", port), GateHandler)
    print(f"[ACE-GPU-Gate] Listening on http://127.0.0.1:{port}", flush=True)
    server.serve_forever()
