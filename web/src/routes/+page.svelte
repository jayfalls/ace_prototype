<script lang="ts">
	import { onMount } from 'svelte';

	type LayerStatus = {
		layer_id: number;
		code: string;
		name: string;
		role: string;
		state: string;
		summary: string;
		last_thought: string;
		live_stream: string;
		active_directive: string;
		northbound_signal: string;
		last_action: string;
		confidence: number;
		model: string;
		last_latency_ms: number;
		total_invocations: number;
		inner_loop_step?: number;
		inner_loop_max?: number;
		updated_at: string;
	};

	type GlobalLoopStatus = {
		name: string;
		frequency: string;
		state: string;
		last_summary: string;
		run_count: number;
		updated_at: string;
	};

	type TelemetryMetrics = {
		total_spans: number;
		total_prompt_tokens: number;
		total_cached_tokens?: number;
		total_completion_tokens: number;
		total_tool_calls: number;
		safety_checks_passed: number;
		safety_interventions: number;
		avg_layer_latency_ms: Record<string, number>;
		spans_by_operation: Record<string, number>;
	};

	type TelemetrySpan = {
		span_id: string;
		trace_id: string;
		parent_span_id?: string;
		layer_code: string;
		operation: string;
		model?: string;
		status: string;
		duration_ms: number;
		prompt_tokens: number;
		completion_tokens: number;
		detail: string;
		created_at: string;
	};

	type SandboxStatus = {
		healthy: boolean;
		os_root_active: boolean;
		traversal_blocked: boolean;
		bwrap_available: boolean;
		bwrap_version: string;
		home_hidden: boolean;
		network_unshared: boolean;
		sandbox_dir: string;
		verification_log: string;
		last_verified_at: string;
	};

	type EngineStatus = {
		agent_id: string;
		mode: string;
		auto_interval_seconds: number;
		active_model: string;
		fallback_models: string[];
		busy: boolean;
		current_cycle_id: string;
		current_layer: number;
		pending_queue_depth: number;
		total_cycles: number;
		total_thoughts: number;
		layers: LayerStatus[];
		global_loops?: GlobalLoopStatus[];
		metrics?: TelemetryMetrics;
		sandbox: SandboxStatus;
	};

	type ModelInfo = {
		id: string;
		name: string;
		provider: string;
		modalities: string[];
		reasoning: boolean;
		context_size: number;
		output_size: number;
	};

	type ChatMessage = {
		id: string;
		role: string;
		source: string;
		content: string;
		reasoning: string;
		model: string;
		latency_ms: number;
		cycle_id: string;
		created_at: string;
	};

	type ThoughtTrace = {
		id: string;
		cycle_id: string;
		layer_id: number;
		layer_code: string;
		loop_kind: string;
		model: string;
		trigger_summary: string;
		system_prompt: string;
		user_prompt: string;
		raw_reasoning: string;
		raw_response: string;
		thought: string;
		state_summary: string;
		southbound: string;
		northbound: string;
		action_type: string;
		action_target: string;
		action_result: string;
		confidence: number;
		latency_ms: number;
		prompt_tokens: number;
		completion_tokens: number;
		created_at: string;
	};

	type BusEnvelope = {
		id: string;
		cycle_id: string;
		topic: string;
		direction: string;
		from_layer: number;
		to_layer: number;
		kind: string;
		summary: string;
		payload: string;
		created_at: string;
	};

	type MemoryEntry = {
		id: string;
		layer_code: string;
		tier?: string;
		category: string;
		title: string;
		content: string;
		tags: string;
		updated_at: string;
	};

	type WikiPageMeta = {
		path: string;
		title: string;
		size_bytes: number;
		updated_at: string;
	};

	type WorkspaceFile = {
		path: string;
		size_bytes: number;
		updated_at: string;
	};

	type ExecResult = {
		command: string;
		stdout: string;
		stderr: string;
		exit_code: number;
		duration_ms: number;
		isolated: boolean;
		executed_at: string;
	};

	type TelegramStatus = {
		enabled: boolean;
		connected: boolean;
		bot_username: string;
		bot_first_name: string;
		masked_token: string;
		allowed_chat_ids: string;
		last_chat_id: number;
		forward_proactive: boolean;
		messages_in: number;
		messages_out: number;
		last_error?: string;
	};

	let engine = $state<EngineStatus | null>(null);
	let models = $state<ModelInfo[]>([]);
	let telegram = $state<TelegramStatus | null>(null);
	let tgTokenInput = $state('');
	let tgAllowedInput = $state('');
	let tgForwardProactive = $state(true);
	let tgSaving = $state(false);
	let tgFeedback = $state('');
	let chatMessages = $state<ChatMessage[]>([]);
	let thoughts = $state<ThoughtTrace[]>([]);
	let telemetrySpans = $state<TelemetrySpan[]>([]);
	let busEvents = $state<BusEnvelope[]>([]);
	let memories = $state<MemoryEntry[]>([]);
	let wikiPages = $state<WikiPageMeta[]>([]);
	let workspaceFiles = $state<WorkspaceFile[]>([]);
	let recentExecs = $state<ExecResult[]>([]);
	let v07Cockpit = $state<any>(null);

	// 3D Conceptual Architecture & Flow Canvas State
	let centerViewMode = $state<'3d' | 'cards'>('3d');
	let canvas3DEl = $state<HTMLCanvasElement | null>(null);
	let camYaw = $state<number>(0.62);
	let camPitch = $state<number>(0.42);
	let camZoom = $state<number>(1.0);
	let explodeZ = $state<number>(1.0);
	let autoOrbit3D = $state<boolean>(true);
	let isDragging3D = $state<boolean>(false);
	let dragLastX = $state<number>(0);
	let dragLastY = $state<number>(0);
	let selected3DItem = $state<any>(null);
	let hitRegions3D: Array<{ x: number; y: number; r: number; kind: string; data: any }> = [];

	let sseConnected = $state(false);
	let fontScale = $state<number>(105); // 95, 105, 115
	let followLiveThoughts = $state(true);

	let chatInput = $state('');
	let chatSending = $state(false);
	let streamingChatContent = $state('');
	let streamingChatReasoning = $state('');

	let rightTab = $state<'explainer' | 'thoughts' | 'telem' | 'bus' | 'wiki' | 'memory' | 'sandbox' | 'telegram'>('explainer');
	let thoughtLayerFilter = $state<number>(-1);
	let selectedThought = $state<ThoughtTrace | null>(null);
	let showRawPrompts = $state(false);

	let activeLayerStimulus = $state<Record<number, string>>({});
	let openDirectiveInput = $state<Record<number, boolean>>({});
	let expandedLayerDetails = $state<Record<number, boolean>>({ 1: true, 2: true, 3: true, 4: true, 5: true, 6: true });
	let steppingLayer = $state<number>(0);

	// Wiki state
	let selectedWikiPath = $state('soul.md');
	let wikiContent = $state('');
	let wikiSaving = $state(false);
	let newWikiPath = $state('');

	// Memory state
	let memoryQuery = $state('');
	let newMemTitle = $state('');
	let newMemContent = $state('');
	let newMemTags = $state('goal,ace');

	// Sandbox state
	let sandboxCmd = $state('uname -a && ls -la /workspace /wiki && (ls /home 2>&1 || true)');
	let sandboxRunning = $state(false);
	let selectedWorkspaceFile = $state('');
	let workspaceFileContent = $state('');

	const layerStyles: Record<number, { border: string; badge: string; accent: string }> = {
		1: { border: 'border-amber-500/40', badge: 'bg-amber-500/20 text-amber-200 border-amber-400/50', accent: 'text-amber-300' },
		2: { border: 'border-violet-500/40', badge: 'bg-violet-500/20 text-violet-200 border-violet-400/50', accent: 'text-violet-300' },
		3: { border: 'border-cyan-500/40', badge: 'bg-cyan-500/20 text-cyan-200 border-cyan-400/50', accent: 'text-cyan-300' },
		4: { border: 'border-blue-500/40', badge: 'bg-blue-500/20 text-blue-200 border-blue-400/50', accent: 'text-blue-300' },
		5: { border: 'border-emerald-500/40', badge: 'bg-emerald-500/20 text-emerald-200 border-emerald-400/50', accent: 'text-emerald-300' },
		6: { border: 'border-rose-500/40', badge: 'bg-rose-500/20 text-rose-200 border-rose-400/50', accent: 'text-rose-300' }
	};

	// Splits dense walls of text into scannable bullet points / paragraphs for effortless reading
	function formatReadableBlocks(raw: string | undefined): string[] {
		if (!raw) return [];
		const text = raw.trim();
		if (!text) return [];

		// If already multiline with bullets or paragraphs
		if (text.includes('\n')) {
			return text
				.split('\n')
				.map((s) => s.trim())
				.filter(Boolean);
		}

		// Split on numbered markers like "(1)", "(2)", "(Step 1)", "(a)", "; (1)" or sentence boundaries if long
		const stepSplit = text
			.replace(/\s*\((?:Step\s*)?\d+\)\s*/gi, '\n• ')
			.replace(/\s*\([a-e]\)\s*/g, '\n  ◦ ')
			.split('\n')
			.map((s) => s.trim())
			.filter(Boolean);

		if (stepSplit.length > 1) {
			return stepSplit;
		}

		// If it's a long paragraph (> 220 chars), break into 1-sentence readable bullets
		if (text.length > 220) {
			const sentences = text
				.split(/(?<=[.!?])\s+(?=[A-Z0-9"(\[])/)
				.map((s) => s.trim())
				.filter(Boolean);
			if (sentences.length > 1) {
				return sentences.map((s) => (s.startsWith('•') ? s : `• ${s}`));
			}
		}

		return [text];
	}

	function cleanLiveStream(raw: string | undefined): string {
		if (!raw) return '';
		// Strip trailing raw JSON scaffolding noise if possible so user reads clean thoughts
		return raw.replace(/^\s*\{\s*"thought"\s*:\s*"/, '').trim();
	}

	function setFontScale(scale: number) {
		fontScale = scale;
		document.documentElement.style.fontSize = `${scale}%`;
		try {
			localStorage.setItem('ace-font-scale', String(scale));
		} catch {}
	}

	async function loadFullState() {
		const res = await fetch('/api/state');
		if (!res.ok) return;
		const data = await res.json();
		engine = data.engine;
		models = data.models || [];
		if (data.telegram) {
			telegram = data.telegram;
			if (!tgAllowedInput && data.telegram.allowed_chat_ids) {
				tgAllowedInput = data.telegram.allowed_chat_ids;
			}
			tgForwardProactive = data.telegram.forward_proactive ?? true;
		}
		chatMessages = data.chat_messages || [];
		thoughts = data.thoughts || [];
		if (!selectedThought && thoughts.length > 0) {
			selectedThought = thoughts[0];
		}
		telemetrySpans = data.telemetry_spans || [];
		busEvents = data.bus_events || [];
		memories = data.memories || [];
		wikiPages = data.wiki_pages || [];
		workspaceFiles = data.workspace_files || [];
		recentExecs = data.recent_execs || [];
		if (data.v07_cockpit) {
			v07Cockpit = data.v07_cockpit;
			if (!selected3DItem && data.v07_cockpit.layer_pyramid?.length) {
				selected3DItem = { kind: 'layer', data: data.v07_cockpit.layer_pyramid[5] };
			}
		}
	}

	function project3D(
		x: number,
		y: number,
		z: number,
		w: number,
		h: number
	): { sx: number; sy: number; depth: number; scale: number } {
		const cosY = Math.cos(camYaw);
		const sinY = Math.sin(camYaw);
		const cosP = Math.cos(camPitch);
		const sinP = Math.sin(camPitch);
		const rx = x * cosY - y * sinY;
		const ry = x * sinY + y * cosY;
		const rz = z * explodeZ;
		const py = ry * cosP - rz * sinP;
		const pz = ry * sinP + rz * cosP;
		const fov = 560 * camZoom;
		const dist = 520 + py;
		const s = fov / Math.max(120, dist);
		return {
			sx: w * 0.5 + rx * s,
			sy: h * 0.52 - pz * s,
			depth: py,
			scale: s
		};
	}

	function draw3DBox(
		ctx: CanvasRenderingContext2D,
		cx: number,
		cy: number,
		cz: number,
		wx: number,
		wy: number,
		wz: number,
		topColor: string,
		sideColor: string,
		strokeColor: string,
		w: number,
		h: number
	) {
		const hx = wx * 0.5;
		const hy = wy * 0.5;
		const hz = wz * 0.5;
		const c = [
			project3D(cx - hx, cy - hy, cz - hz, w, h),
			project3D(cx + hx, cy - hy, cz - hz, w, h),
			project3D(cx + hx, cy + hy, cz - hz, w, h),
			project3D(cx - hx, cy + hy, cz - hz, w, h),
			project3D(cx - hx, cy - hy, cz + hz, w, h),
			project3D(cx + hx, cy - hy, cz + hz, w, h),
			project3D(cx + hx, cy + hy, cz + hz, w, h),
			project3D(cx - hx, cy + hy, cz + hz, w, h)
		];
		const faces = [
			{ idx: [0, 1, 5, 4], fill: sideColor },
			{ idx: [1, 2, 6, 5], fill: sideColor },
			{ idx: [2, 3, 7, 6], fill: sideColor },
			{ idx: [3, 0, 4, 7], fill: sideColor },
			{ idx: [4, 5, 6, 7], fill: topColor }
		];
		faces.sort((a, b) => {
			const da = a.idx.reduce((acc, i) => acc + c[i].depth, 0) / 4;
			const db = b.idx.reduce((acc, i) => acc + c[i].depth, 0) / 4;
			return db - da;
		});
		for (const f of faces) {
			ctx.beginPath();
			ctx.moveTo(c[f.idx[0]].sx, c[f.idx[0]].sy);
			for (let k = 1; k < f.idx.length; k++) {
				ctx.lineTo(c[f.idx[k]].sx, c[f.idx[k]].sy);
			}
			ctx.closePath();
			ctx.fillStyle = f.fill;
			ctx.fill();
			ctx.strokeStyle = strokeColor;
			ctx.lineWidth = 1.25;
			ctx.stroke();
		}
	}

	function render3DArchitectureScene(nowMs: number) {
		if (!canvas3DEl || centerViewMode !== '3d') return;
		const ctx = canvas3DEl.getContext('2d');
		if (!ctx) return;
		const rect = canvas3DEl.getBoundingClientRect();
		const dpr = window.devicePixelRatio || 1;
		if (canvas3DEl.width !== Math.floor(rect.width * dpr) || canvas3DEl.height !== Math.floor(rect.height * dpr)) {
			canvas3DEl.width = Math.max(300, Math.floor(rect.width * dpr));
			canvas3DEl.height = Math.max(260, Math.floor(rect.height * dpr));
		}
		ctx.save();
		ctx.scale(dpr, dpr);
		const w = rect.width;
		const h = rect.height;

		if (autoOrbit3D && !isDragging3D) {
			camYaw += 0.0028;
		}

		// Deep space grid background
		ctx.fillStyle = '#070b14';
		ctx.fillRect(0, 0, w, h);

		// Draw 3D base reference grid at z = -135
		ctx.strokeStyle = 'rgba(51, 65, 85, 0.35)';
		ctx.lineWidth = 1;
		for (let g = -200; g <= 200; g += 40) {
			const p1 = project3D(g, -200, -135, w, h);
			const p2 = project3D(g, 200, -135, w, h);
			ctx.beginPath();
			ctx.moveTo(p1.sx, p1.sy);
			ctx.lineTo(p2.sx, p2.sy);
			ctx.stroke();
			const q1 = project3D(-200, g, -135, w, h);
			const q2 = project3D(200, g, -135, w, h);
			ctx.beginPath();
			ctx.moveTo(q1.sx, q1.sy);
			ctx.lineTo(q2.sx, q2.sy);
			ctx.stroke();
		}

		hitRegions3D = [];
		const pyramid = v07Cockpit?.layer_pyramid || [];
		const layers = engine?.layers || [];

		const layerMeta: Array<{
			id: number;
			code: string;
			z: number;
			baseSize: number;
			topFill: string;
			sideFill: string;
			stroke: string;
		}> = [
			{ id: 6, code: 'L6', z: -105, baseSize: 156, topFill: 'rgba(225, 29, 72, 0.34)', sideFill: 'rgba(159, 18, 57, 0.45)', stroke: '#fb7185' },
			{ id: 5, code: 'L5', z: -60, baseSize: 118, topFill: 'rgba(16, 185, 129, 0.32)', sideFill: 'rgba(6, 95, 70, 0.45)', stroke: '#34d399' },
			{ id: 4, code: 'L4', z: -15, baseSize: 102, topFill: 'rgba(59, 130, 246, 0.32)', sideFill: 'rgba(30, 64, 175, 0.45)', stroke: '#60a5fa' },
			{ id: 3, code: 'L3', z: 30, baseSize: 88, topFill: 'rgba(6, 182, 212, 0.32)', sideFill: 'rgba(14, 116, 144, 0.45)', stroke: '#22d3ee' },
			{ id: 2, code: 'L2', z: 75, baseSize: 74, topFill: 'rgba(139, 92, 246, 0.32)', sideFill: 'rgba(91, 33, 182, 0.45)', stroke: '#a78bfa' },
			{ id: 1, code: 'L1', z: 120, baseSize: 58, topFill: 'rgba(245, 158, 11, 0.34)', sideFill: 'rgba(146, 64, 14, 0.45)', stroke: '#fbbf24' }
		];

		// Sort layer decks back-to-front by projected center depth
		const sortedDecks = [...layerMeta].sort((a, b) => {
			return project3D(0, 0, b.z, w, h).depth - project3D(0, 0, a.z, w, h).depth;
		});

		// Draw Southbound (Cyan) and Northbound (Amber) 3D Bus Spines
		const sbTop = project3D(-24, -24, 125, w, h);
		const sbBot = project3D(-24, -24, -110, w, h);
		ctx.strokeStyle = 'rgba(34, 211, 238, 0.55)';
		ctx.lineWidth = 2.5;
		ctx.beginPath();
		ctx.moveTo(sbTop.sx, sbTop.sy);
		ctx.lineTo(sbBot.sx, sbBot.sy);
		ctx.stroke();

		const nbTop = project3D(24, 24, 125, w, h);
		const nbBot = project3D(24, 24, -110, w, h);
		ctx.strokeStyle = 'rgba(251, 191, 36, 0.55)';
		ctx.lineWidth = 2.5;
		ctx.beginPath();
		ctx.moveTo(nbBot.sx, nbBot.sy);
		ctx.lineTo(nbTop.sx, nbTop.sy);
		ctx.stroke();

		// Animated Bus Particles along Southbound (top->down) & Northbound (bottom->up)
		for (let p = 0; p < 4; p++) {
			const tSb = ((nowMs * 0.00045 + p * 0.25) % 1);
			const zSb = 120 - tSb * 225;
			const ptSb = project3D(-24, -24, zSb, w, h);
			ctx.fillStyle = '#22d3ee';
			ctx.beginPath();
			ctx.arc(ptSb.sx, ptSb.sy, 4.2 * ptSb.scale, 0, Math.PI * 2);
			ctx.fill();

			const tNb = ((nowMs * 0.00045 + p * 0.25 + 0.12) % 1);
			const zNb = -105 + tNb * 225;
			const ptNb = project3D(24, 24, zNb, w, h);
			ctx.fillStyle = '#fbbf24';
			ctx.beginPath();
			ctx.arc(ptNb.sx, ptNb.sy, 4.2 * ptNb.scale, 0, Math.PI * 2);
			ctx.fill();
		}

		// Draw 3D Layer Decks (Pyramidal Hierarchy L1 < L2 < L3 < L4 << L6)
		for (const lm of sortedDecks) {
			const pStat = pyramid.find((p: any) => p.layer_code === lm.code) || {
				layer_code: lm.code,
				traces: 0,
				share_pct: lm.id === 6 ? 70 : 6,
				cache_hit_pct: 80,
				avg_latency_ms: 0
			};
			const lStat = layers.find((l) => l.layer_id === lm.id);
			const isActive = lStat?.state === 'thinking' || engine?.current_layer === lm.id;
			const dynamicScale = 0.82 + Math.min(0.55, (pStat.share_pct || 0) / 110);
			const wx = lm.baseSize * dynamicScale;
			const wy = lm.baseSize * dynamicScale;
			const wz = isActive ? 20 : 14;

			draw3DBox(
				ctx,
				0,
				0,
				lm.z,
				wx,
				wy,
				wz,
				isActive ? 'rgba(167, 139, 250, 0.65)' : lm.topFill,
				lm.sideFill,
				isActive ? '#ffffff' : lm.stroke,
				w,
				h
			);

			const labelPt = project3D(0, 0, lm.z + wz * 0.6, w, h);
			ctx.fillStyle = '#f8fafc';
			ctx.font = `bold ${Math.max(10, Math.round(11 * labelPt.scale))}px ui-monospace, SFMono-Regular, monospace`;
			ctx.textAlign = 'center';
			const shareLabel = lm.code === 'L5' ? `4-bit GPU (${pStat.traces}×)` : `${ pStat.traces }× (${(pStat.share_pct || 0).toFixed(0)}%)`;
			ctx.fillText(`${lm.code} · ${shareLabel}`, labelPt.sx, labelPt.sy);

			hitRegions3D.push({
				x: labelPt.sx,
				y: labelPt.sy,
				r: Math.max(24, wx * 0.42 * labelPt.scale),
				kind: 'layer',
				data: { ...pStat, layerStatus: lStat }
			});
		}

		// Draw 3D Orbital Connectome & Wiki-Skill Constellation Nodes
		const nodes = (v07Cockpit?.connectome_nodes || []).slice(0, 26);
		nodes.forEach((nd: any, idx: number) => {
			const angle = (idx / Math.max(1, nodes.length)) * Math.PI * 2 + nowMs * 0.00012;
			const ownerL = Math.min(6, Math.max(1, Number(nd.owner_layer) || 6));
			const targetDeck = layerMeta.find((m) => m.id === ownerL) || layerMeta[0];
			const radius = 148 + (idx % 3) * 26;
			const nx = Math.cos(angle) * radius;
			const ny = Math.sin(angle) * radius;
			const nz = targetDeck.z + ((idx % 2 === 0 ? 1 : -1) * 10);

			const nPt = project3D(nx, ny, nz, w, h);
			const deckPt = project3D(0, 0, targetDeck.z, w, h);

			// Tether line from Connectome node to its owning Layer deck
			ctx.strokeStyle =
				nd.kind === 'SKILL'
					? 'rgba(52, 211, 153, 0.30)'
					: nd.kind === 'CONTRADICTION'
						? 'rgba(251, 113, 133, 0.55)'
						: 'rgba(148, 163, 184, 0.20)';
			ctx.lineWidth = 1;
			ctx.beginPath();
			ctx.moveTo(nPt.sx, nPt.sy);
			ctx.lineTo(deckPt.sx, deckPt.sy);
			ctx.stroke();

			const nodeColor =
				nd.kind === 'SKILL'
					? '#10b981'
					: nd.kind === 'CONTRADICTION'
						? '#f43f5e'
						: nd.kind === 'HYPOTHESIS'
							? '#a855f7'
							: '#38bdf8';
			const nr = Math.max(4.5, (nd.kind === 'SKILL' ? 7.5 : 5.8) * nPt.scale);
			ctx.fillStyle = nodeColor;
			ctx.beginPath();
			ctx.arc(nPt.sx, nPt.sy, nr, 0, Math.PI * 2);
			ctx.fill();
			ctx.strokeStyle = '#0f172a';
			ctx.lineWidth = 1.2;
			ctx.stroke();

			if (idx < 12 || nd.kind === 'SKILL' || nd.kind === 'CONTRADICTION') {
				ctx.fillStyle = '#cbd5e1';
				ctx.font = `${Math.max(8, Math.round(9 * nPt.scale))}px ui-monospace, monospace`;
				ctx.textAlign = 'left';
				const shortName = (nd.path || nd.id || '').split('/').pop()?.slice(0, 22) || nd.kind;
				ctx.fillText(shortName, nPt.sx + nr + 3, nPt.sy + 3);
			}

			hitRegions3D.push({
				x: nPt.sx,
				y: nPt.sy,
				r: nr + 8,
				kind: 'node',
				data: nd
			});
		});

		ctx.restore();
	}

	function handleCanvasMouseDown(e: MouseEvent) {
		isDragging3D = true;
		dragLastX = e.clientX;
		dragLastY = e.clientY;
	}

	function handleCanvasMouseMove(e: MouseEvent) {
		if (!isDragging3D) return;
		const dx = e.clientX - dragLastX;
		const dy = e.clientY - dragLastY;
		dragLastX = e.clientX;
		dragLastY = e.clientY;
		camYaw += dx * 0.008;
		camPitch = Math.max(-0.25, Math.min(1.25, camPitch + dy * 0.008));
	}

	function handleCanvasMouseUp(e: MouseEvent) {
		if (isDragging3D && Math.abs(e.clientX - dragLastX) < 4 && Math.abs(e.clientY - dragLastY) < 4 && canvas3DEl) {
			const rect = canvas3DEl.getBoundingClientRect();
			const mx = e.clientX - rect.left;
			const my = e.clientY - rect.top;
			for (let i = hitRegions3D.length - 1; i >= 0; i--) {
				const hr = hitRegions3D[i];
				if (Math.hypot(mx - hr.x, my - hr.y) <= hr.r) {
					selected3DItem = { kind: hr.kind, data: hr.data };
					if (hr.kind === 'layer' && hr.data?.layerStatus) {
						thoughtLayerFilter = hr.data.layerStatus.layer_id;
						const latest = thoughts.find((t) => t.layer_id === hr.data.layerStatus.layer_id);
						if (latest) selectedThought = latest;
					}
					break;
				}
			}
		}
		isDragging3D = false;
	}

	function handleCanvasWheel(e: WheelEvent) {
		e.preventDefault();
		camZoom = Math.max(0.55, Math.min(1.85, camZoom - e.deltaY * 0.001));
	}

	async function loadWikiPage(path: string) {
		selectedWikiPath = path;
		const res = await fetch(`/api/wiki/read?path=${encodeURIComponent(path)}`);
		if (res.ok) {
			const data = await res.json();
			wikiContent = data.content;
		}
	}

	async function saveWikiPage() {
		const target = newWikiPath.trim() || selectedWikiPath;
		if (!target) return;
		wikiSaving = true;
		try {
			const res = await fetch('/api/wiki/write', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ path: target, content: wikiContent })
			});
			if (res.ok) {
				const data = await res.json();
				wikiPages = data.wiki_pages || wikiPages;
				selectedWikiPath = target.endsWith('.md') ? target : target + '.md';
				newWikiPath = '';
			}
		} finally {
			wikiSaving = false;
		}
	}

	async function sendChat(customText?: string) {
		const text = (customText ?? chatInput).trim();
		if (!text || chatSending) return;
		chatInput = '';
		chatSending = true;
		streamingChatContent = '';
		streamingChatReasoning = '';
		try {
			await fetch('/api/chat', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ content: text })
			});
		} finally {
			chatSending = false;
			streamingChatContent = '';
			streamingChatReasoning = '';
		}
	}

	async function setEngineMode(mode: string) {
		const res = await fetch('/api/engine/mode', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ mode, interval_seconds: engine?.auto_interval_seconds || 45 })
		});
		if (res.ok) engine = await res.json();
	}

	async function triggerCascade(startLayer = 1, maxHops = 6, customTrigger = '') {
		await fetch('/api/engine/cycle', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ start_layer: startLayer, max_hops: maxHops, trigger: customTrigger })
		});
	}

	async function stepSingleLayer(layerId: number) {
		steppingLayer = layerId;
		const stimulus = activeLayerStimulus[layerId] || '';
		try {
			const res = await fetch('/api/engine/step', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ layer_id: layerId, stimulus })
			});
			if (res.ok) {
				const trace: ThoughtTrace = await res.json();
				selectedThought = trace;
				rightTab = 'thoughts';
			}
		} finally {
			steppingLayer = 0;
		}
	}

	async function stopEngine() {
		const res = await fetch('/api/engine/stop', { method: 'POST' });
		if (res.ok) engine = await res.json();
	}

	async function selectModel(modelId: string) {
		const res = await fetch('/api/models/select', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ model_id: modelId })
		});
		if (res.ok) engine = await res.json();
	}

	async function searchMemories() {
		const res = await fetch(`/api/memory/search?q=${encodeURIComponent(memoryQuery)}`);
		if (res.ok) memories = await res.json();
	}

	async function addMemory() {
		if (!newMemTitle.trim() || !newMemContent.trim()) return;
		const res = await fetch('/api/memory/remember', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				layer_code: 'USER',
				category: 'knowledge',
				title: newMemTitle,
				content: newMemContent,
				tags: newMemTags
			})
		});
		if (res.ok) {
			newMemTitle = '';
			newMemContent = '';
			await searchMemories();
		}
	}

	async function runSandboxCommand() {
		if (!sandboxCmd.trim() || sandboxRunning) return;
		sandboxRunning = true;
		try {
			const res = await fetch('/api/sandbox/exec', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ command: sandboxCmd })
			});
			if (res.ok) {
				const data = await res.json();
				workspaceFiles = data.workspace_files || workspaceFiles;
			}
		} finally {
			sandboxRunning = false;
		}
	}

	async function readWorkspaceFile(path: string) {
		selectedWorkspaceFile = path;
		const res = await fetch(`/api/sandbox/file?path=${encodeURIComponent(path)}`);
		if (res.ok) {
			const data = await res.json();
			workspaceFileContent = data.content;
		}
	}

	async function reverifySandbox() {
		const res = await fetch('/api/sandbox/verify', { method: 'POST' });
		if (res.ok && engine) {
			engine.sandbox = await res.json();
		}
	}

	async function saveTelegramConfig(enabled: boolean) {
		tgSaving = true;
		tgFeedback = '';
		try {
			const res = await fetch('/api/telegram/configure', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					token: tgTokenInput,
					allowed_chat_ids: tgAllowedInput,
					forward_proactive: tgForwardProactive,
					enabled
				})
			});
			const data = await res.json();
			if (!res.ok) {
				if (data.telegram) telegram = data.telegram;
				tgFeedback = 'Error: ' + (data.error || 'Failed to connect bot');
			} else {
				telegram = data;
				tgTokenInput = '';
				tgFeedback = data.connected ? `Connected @${data.bot_username}!` : 'Telegram bot disconnected.';
			}
		} finally {
			tgSaving = false;
		}
	}

	async function sendTelegramTestPing() {
		tgFeedback = 'Sending test ping...';
		const res = await fetch('/api/telegram/test', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({})
		});
		const data = await res.json();
		if (!res.ok) {
			tgFeedback = 'Error: ' + (data.error || 'Could not send ping');
		} else {
			telegram = data;
			tgFeedback = `Test ping sent to Chat ID ${data.last_chat_id || data.allowed_chat_ids}!`;
		}
	}

	function connectSSE() {
		const es = new EventSource('/api/events');
		es.onopen = () => {
			sseConnected = true;
		};
		es.onerror = () => {
			sseConnected = false;
		};
		es.onmessage = (e) => {
			try {
				const evt = JSON.parse(e.data);
				const p = evt.payload;
				switch (evt.type) {
					case 'engine.status':
						engine = p;
						break;
					case 'layer.status':
						if (engine) {
							engine.layers = engine.layers.map((l) => (l.layer_id === p.layer_id ? p : l));
						}
						break;
					case 'layer.stream':
						if (engine) {
							engine.layers = engine.layers.map((l) =>
								l.layer_id === p.layer_id
									? { ...l, state: 'thinking', live_stream: (l.live_stream || '') + p.delta }
									: l
							);
						}
						break;
					case 'chat.delta':
						if (p.content_delta) streamingChatContent += p.content_delta;
						if (p.reasoning_delta) streamingChatReasoning += p.reasoning_delta;
						break;
					case 'chat.message':
						if (p.role === 'assistant') {
							streamingChatContent = '';
							streamingChatReasoning = '';
						}
						if (!chatMessages.some((m) => m.id === p.id)) {
							chatMessages = [...chatMessages, p];
						}
						break;
					case 'thought.recorded':
						thoughts = [p, ...thoughts.filter((t) => t.id !== p.id)].slice(0, 80);
						if (followLiveThoughts || !selectedThought) {
							selectedThought = p;
						}
						break;
					case 'telemetry.span':
						telemetrySpans = [p, ...telemetrySpans.filter((s) => s.span_id !== p.span_id)].slice(0, 120);
						break;
					case 'bus.envelope':
						busEvents = [p, ...busEvents.filter((b) => b.id !== p.id)].slice(0, 80);
						break;
					case 'memory.recorded':
						memories = [p, ...memories.filter((m) => m.id !== p.id)].slice(0, 50);
						break;
					case 'wiki.updated':
						loadFullState();
						if (p.path === selectedWikiPath) loadWikiPage(selectedWikiPath);
						break;
					case 'sandbox.updated':
						loadFullState();
						break;
					case 'sandbox.exec':
						recentExecs = [p, ...recentExecs].slice(0, 25);
						loadFullState();
						break;
					case 'telegram.status':
						telegram = p;
						break;
				}
			} catch {}
		};
		return es;
	}

	let filteredThoughts = $derived(
		thoughtLayerFilter === -1
			? thoughts
			: thoughts.filter((t) => t.layer_id === thoughtLayerFilter)
	);

	onMount(() => {
		try {
			const savedScale = Number(localStorage.getItem('ace-font-scale'));
			if (savedScale >= 90 && savedScale <= 125) {
				setFontScale(savedScale);
			} else {
				setFontScale(105);
			}
		} catch {
			setFontScale(105);
		}
		loadFullState();
		loadWikiPage('soul.md');
		const es = connectSSE();
		let animId = 0;
		const loop = (t: number) => {
			render3DArchitectureScene(t);
			animId = requestAnimationFrame(loop);
		};
		animId = requestAnimationFrame(loop);
		return () => {
			cancelAnimationFrame(animId);
			es.close();
		};
	});
</script>

<div class="flex h-screen w-screen flex-col overflow-hidden bg-[#090d16] text-slate-100">
	<!-- TOP COMMAND BAR -->
	<header class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-800 bg-slate-950 px-5 py-3">
		<div class="flex items-center gap-3.5">
			<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-violet-600 to-indigo-600 text-base font-extrabold tracking-wider text-white shadow-lg shadow-violet-600/30">
				ACE
			</div>
			<div>
				<div class="flex items-center gap-2.5">
					<h1 class="text-base font-bold tracking-wide text-white">ACE COGNITIVE COCKPIT</h1>
					<span class="flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold {sseConnected ? 'border-emerald-500/40 bg-emerald-500/15 text-emerald-300' : 'border-amber-500/40 bg-amber-500/15 text-amber-300'}">
						<span class="h-2 w-2 rounded-full {sseConnected ? 'bg-emerald-400 animate-pulse' : 'bg-amber-400'}"></span>
						{sseConnected ? 'LIVE' : 'RECONNECTING'}
					</span>
				</div>
				<p class="text-xs text-slate-300">
					Cycles: <strong class="text-white">{engine?.total_cycles ?? 0}</strong> ·
					Traces: <strong class="text-white">{engine?.total_thoughts ?? 0}</strong> ·
					Queue: <strong class="text-white">{engine?.pending_queue_depth ?? 0}</strong>
				</p>
			</div>
		</div>

		<!-- CONTROLS -->
		<div class="flex flex-wrap items-center gap-2.5">
			<!-- Text Size Switcher -->
			<div class="flex items-center rounded-lg border border-slate-700 bg-slate-900 p-0.5 text-xs" title="Adjust UI text size">
				{#each [
					{ scale: 95, label: 'A-' },
					{ scale: 105, label: 'A' },
					{ scale: 115, label: 'A+' }
				] as fs}
					<button
						onclick={() => setFontScale(fs.scale)}
						class="rounded-md px-2 py-1 font-bold transition {fontScale === fs.scale
							? 'bg-slate-700 text-white'
							: 'text-slate-400 hover:text-white'}"
					>
						{fs.label}
					</button>
				{/each}
			</div>

			<!-- Sandbox Security Badge -->
			<button
				onclick={() => (rightTab = 'sandbox')}
				class="flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-semibold transition {engine?.sandbox?.healthy
					? 'border-emerald-500/50 bg-emerald-950/60 text-emerald-200 hover:bg-emerald-900/60'
					: 'border-rose-500/50 bg-rose-950/60 text-rose-200'}"
				title={engine?.sandbox?.verification_log || 'Inspect sandbox isolation'}
			>
				<span>🛡️</span>
				<span>{engine?.sandbox?.healthy ? 'Sandbox Jailed' : 'Sandbox Warning'}</span>
			</button>

			<!-- Data Model & Gating Viz Link -->
			<a
				href="/showcase/model-viz.html"
				target="_blank"
				rel="noopener noreferrer"
				class="flex items-center gap-1.5 rounded-lg border border-violet-500/50 bg-violet-950/60 px-3 py-1.5 text-xs font-semibold text-violet-200 transition hover:bg-violet-900/60"
				title="Open Empirical Telemetry Data Model, Gating Analysis & Hermes/OpenClaw Comparison"
			>
				<span>📊</span>
				<span>Data Model & Gates Viz</span>
			</a>

			<!-- Telegram Channel Badge -->
			<button
				onclick={() => (rightTab = 'telegram')}
				class="flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-semibold transition {telegram?.connected
					? 'border-sky-500/50 bg-sky-950/60 text-sky-200 hover:bg-sky-900/60'
					: 'border-slate-700 bg-slate-900 text-slate-300 hover:border-slate-600 hover:text-white'}"
				title="Configure Telegram Bot Chat Interface"
			>
				<span>📱</span>
				<span>{telegram?.connected ? `@${telegram.bot_username}` : 'Telegram'}</span>
			</button>

			<!-- Execution Mode Pills -->
			<div class="flex items-center rounded-lg border border-slate-700 bg-slate-900 p-0.5 text-xs">
				{#each [
					{ id: 'reactive', label: '⚡ Reactive' },
					{ id: 'manual', label: '🔬 Manual' },
					{ id: 'autonomous', label: '🔄 Autonomous' },
					{ id: 'paused', label: '⏸ Paused' }
				] as m}
					<button
						onclick={() => setEngineMode(m.id)}
						class="rounded-md px-2.5 py-1.5 font-semibold transition {engine?.mode === m.id
							? 'bg-violet-600 text-white shadow'
							: 'text-slate-300 hover:text-white'}"
					>
						{m.label}
					</button>
				{/each}
			</div>

			<!-- Cascade & Stop Buttons -->
			<button
				onclick={() => triggerCascade(1, 6)}
				disabled={engine?.busy}
				class="rounded-lg bg-gradient-to-r from-violet-600 to-indigo-600 px-3.5 py-1.5 text-xs font-bold text-white shadow hover:from-violet-500 hover:to-indigo-500 disabled:opacity-50"
			>
				{engine?.busy ? `Thinking (L${engine.current_layer})...` : '▶ Cascade L1→L6'}
			</button>

			{#if engine?.busy}
				<button
					onclick={stopEngine}
					class="rounded-lg border border-rose-500/60 bg-rose-950/80 px-3 py-1.5 text-xs font-bold text-rose-200 hover:bg-rose-900"
				>
					■ Stop
				</button>
			{/if}

			<!-- OpenCode Free Model Selector -->
			<select
				value={engine?.active_model || 'deepseek-v4.1-flash-free'}
				onchange={(e) => selectModel((e.currentTarget as HTMLSelectElement).value)}
				class="rounded-lg border border-slate-700 bg-slate-900 px-3 py-1.5 text-xs font-semibold text-white focus:border-violet-500 focus:outline-none"
			>
				{#each models as m}
					<option value={m.id}>
						{m.name}
					</option>
				{/each}
			</select>
		</div>
	</header>

	<!-- MAIN 3-COLUMN COCKPIT GRID -->
	<div class="grid flex-1 grid-cols-12 overflow-hidden">
		<!-- COLUMN 1 (3 cols): FAST CHAT & PROACTIVE AGENT STREAM -->
		<section class="col-span-3 flex flex-col border-r border-slate-800 bg-slate-950/60 overflow-hidden">
			<div class="flex items-center justify-between border-b border-slate-800 px-4 py-3">
				<div>
					<h2 class="text-sm font-bold text-white">Fast Chat & Proactive Stream</h2>
					<p class="text-xs text-slate-300">Direct dialogue + Northbound L6→L1 sense</p>
				</div>
				<span class="rounded-md bg-slate-800 px-2 py-0.5 text-xs font-semibold text-slate-200">
					{chatMessages.length}
				</span>
			</div>

			<!-- Quick Starter Prompts -->
			<div class="flex flex-col gap-1.5 border-b border-slate-800/80 bg-slate-900/40 px-3.5 py-2.5">
				<div class="text-xs font-semibold text-slate-400">Quick Prompts:</div>
				{#each [
					'Search the live web for profitable B2B micro-SaaS pain points and write notes/market-opportunities.md.',
					'Pick our #1 revenue opportunity and run a Python unit-economics simulator in /workspace/venture_model.py.',
					'Build and test the working MVP prototype in /workspace/mvp_engine.py.'
				] as prompt}
					<button
						onclick={() => sendChat(prompt)}
						disabled={chatSending}
						class="rounded-lg border border-slate-700/80 bg-slate-900/90 px-3 py-1.5 text-left text-xs font-medium text-slate-200 transition hover:border-violet-500 hover:bg-slate-800 hover:text-white disabled:opacity-40"
					>
						{prompt}
					</button>
				{/each}
			</div>

			<!-- Messages List -->
			<div class="flex-1 space-y-3 overflow-y-auto p-3.5">
				{#if chatMessages.length === 0}
					<div class="rounded-xl border border-dashed border-slate-700 bg-slate-900/30 p-5 text-center text-sm leading-relaxed text-slate-300">
						Talk to ACE below. Every message answers immediately via Fast Chat and also broadcasts onto the Northbound bus so Layers L1–L6 can reflect and act.
					</div>
				{/if}

				{#each chatMessages as msg (msg.id)}
					<div
						class="rounded-xl border p-3.5 text-sm leading-relaxed {msg.role === 'user'
							? 'ml-3 border-violet-500/40 bg-violet-950/30 text-white'
							: msg.role === 'proactive'
								? 'border-rose-500/50 bg-rose-950/25 text-white'
								: 'border-slate-700/80 bg-slate-900/90 text-slate-100'}"
					>
						<div class="mb-1.5 flex items-center justify-between text-xs">
							<span class="font-bold uppercase tracking-wide {msg.role === 'user' ? 'text-violet-300' : msg.role === 'proactive' ? 'text-rose-300' : 'text-cyan-300'}">
								{msg.role === 'user' ? 'You' : msg.role === 'proactive' ? `⚡ ${msg.source} Proactive` : 'ACE'}
							</span>
							{#if msg.model}
								<span class="text-slate-400">{(msg.latency_ms / 1000).toFixed(1)}s</span>
							{/if}
						</div>

						{#if msg.reasoning}
							<details class="mb-2 rounded-lg border border-slate-700/80 bg-slate-950/80 p-2.5 text-xs text-slate-300">
								<summary class="cursor-pointer font-semibold text-slate-200">Reasoning Trace</summary>
								<div class="mt-2 whitespace-pre-wrap leading-relaxed text-slate-300">{msg.reasoning}</div>
							</details>
						{/if}

						<div class="whitespace-pre-wrap leading-relaxed">{msg.content}</div>
					</div>
				{/each}

				{#if chatSending || streamingChatContent || streamingChatReasoning}
					<div class="rounded-xl border border-cyan-500/50 bg-slate-900 p-3.5 text-sm">
						<div class="mb-1.5 flex items-center gap-2 text-xs font-bold text-cyan-300">
							<span class="h-2 w-2 animate-ping rounded-full bg-cyan-400"></span>
							<span>ACE Responding...</span>
						</div>
						{#if streamingChatReasoning}
							<div class="mb-2 max-h-32 overflow-y-auto rounded-lg bg-slate-950 p-2.5 text-xs leading-relaxed text-slate-300 whitespace-pre-wrap">{streamingChatReasoning}</div>
						{/if}
						<div class="whitespace-pre-wrap leading-relaxed text-white">{streamingChatContent || 'Thinking...'}</div>
					</div>
				{/if}
			</div>

			<!-- Chat Input Form -->
			<form
				onsubmit={(e) => {
					e.preventDefault();
					sendChat();
				}}
				class="border-t border-slate-800 bg-slate-950 p-3.5"
			>
				<div class="flex gap-2">
					<input
						type="text"
						bind:value={chatInput}
						placeholder="Talk to ACE or assign a mission..."
						class="flex-1 rounded-xl border border-slate-700 bg-slate-900 px-3.5 py-2.5 text-sm text-white placeholder-slate-400 focus:border-violet-500 focus:outline-none"
					/>
					<button
						type="submit"
						disabled={chatSending || !chatInput.trim()}
						class="rounded-xl bg-violet-600 px-4 py-2.5 text-sm font-bold text-white hover:bg-violet-500 disabled:opacity-40"
					>
						Send
					</button>
				</div>
			</form>
		</section>

		<!-- COLUMN 2 (5 cols): INTERACTIVE 3D CONCEPTUAL ARCHITECTURE & 6-LAYER STACK -->
		<section class="col-span-5 flex flex-col border-r border-slate-800 bg-[#0b101d] overflow-hidden">
			<div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-800 px-4 py-3">
				<div>
					<h2 class="text-sm font-bold text-white">
						{centerViewMode === '3d'
							? '🌐 ACE v0.7 Interactive 3D Conceptual Architecture & Flow'
							: '6-Layer Cognitive Stack (L1 → L6)'}
					</h2>
					<p class="text-xs text-slate-300">
						{centerViewMode === '3d'
							? 'Drag to orbit 3D tower · Scroll to zoom · Click any 3D Layer Deck, Skill, or Connectome node'
							: "Click any layer's Traces button to inspect its full reasoning & prompts"}
					</p>
				</div>
				<div class="flex items-center gap-1.5">
					<div class="flex items-center rounded-lg border border-slate-700 bg-slate-900 p-0.5 text-xs">
						<button
							onclick={() => (centerViewMode = '3d')}
							class="rounded-md px-2.5 py-1 font-bold transition {centerViewMode === '3d'
								? 'bg-violet-600 text-white'
								: 'text-slate-300 hover:text-white'}"
						>
							🌐 3D Tower & Flow
						</button>
						<button
							onclick={() => (centerViewMode = 'cards')}
							class="rounded-md px-2.5 py-1 font-bold transition {centerViewMode === 'cards'
								? 'bg-violet-600 text-white'
								: 'text-slate-300 hover:text-white'}"
						>
							📋 Layer Cards
						</button>
					</div>
					{#if centerViewMode === 'cards'}
						<button
							onclick={() => {
								const allOpen = Object.values(expandedLayerDetails).every(Boolean);
								for (let i = 1; i <= 6; i++) expandedLayerDetails[i] = !allOpen;
							}}
							class="rounded-lg border border-slate-700 bg-slate-900 px-2.5 py-1 text-xs font-semibold text-slate-200 hover:bg-slate-800"
						>
							Toggle Directives
						</button>
					{/if}
				</div>
			</div>

			{#if centerViewMode === '3d'}
				<div class="flex flex-1 flex-col overflow-hidden">
					<!-- 3D View Controls Strip -->
					<div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-800 bg-slate-950/90 px-3.5 py-2 text-xs">
						<div class="flex items-center gap-1.5">
							<button
								onclick={() => {
									camYaw = 0.62;
									camPitch = 0.42;
									camZoom = 1.0;
									explodeZ = 1.0;
								}}
								class="rounded border border-slate-700 bg-slate-900 px-2 py-1 font-semibold text-slate-200 hover:bg-slate-800"
							>
								Isometric Rig
							</button>
							<button
								onclick={() => {
									camYaw = 0.0;
									camPitch = 1.18;
									camZoom = 0.95;
								}}
								class="rounded border border-slate-700 bg-slate-900 px-2 py-1 font-semibold text-slate-200 hover:bg-slate-800"
							>
								Top Connectome
							</button>
							<button
								onclick={() => {
									camYaw = 0.15;
									camPitch = 0.08;
									camZoom = 1.05;
								}}
								class="rounded border border-slate-700 bg-slate-900 px-2 py-1 font-semibold text-slate-200 hover:bg-slate-800"
							>
								Pyramid Profile
							</button>
							<label class="ml-1 flex items-center gap-1 text-slate-300">
								<input type="checkbox" bind:checked={autoOrbit3D} class="rounded accent-violet-500" />
								<span>Orbit</span>
							</label>
						</div>
						<div class="flex items-center gap-2">
							<span class="text-slate-400">Explode Z:</span>
							<input
								type="range"
								min="0.6"
								max="1.5"
								step="0.05"
								bind:value={explodeZ}
								class="w-20 accent-violet-500"
							/>
							<span class="rounded bg-emerald-950/80 border border-emerald-500/40 px-2 py-0.5 font-mono text-[11px] text-emerald-300">
								Skills: {v07Cockpit?.wiki_skills?.active_count ?? 0} active
							</span>
						</div>
					</div>

					<!-- Interactive 3D Canvas -->
					<div class="relative flex-1 min-h-[290px] select-none overflow-hidden bg-[#070b14]">
						<canvas
							bind:this={canvas3DEl}
							onmousedown={handleCanvasMouseDown}
							onmousemove={handleCanvasMouseMove}
							onmouseup={handleCanvasMouseUp}
							onmouseleave={() => (isDragging3D = false)}
							onwheel={handleCanvasWheel}
							class="h-full w-full cursor-grab active:cursor-grabbing"
						></canvas>

						<!-- Legend Overlay -->
						<div class="pointer-events-none absolute top-3 left-3 rounded-xl border border-slate-800/90 bg-slate-950/85 p-2.5 text-[11px] space-y-1">
							<div class="font-bold text-white">Pyramidal Hierarchy (L1 &lt; L2 &lt; L3 &lt; L4 &lt;&lt; L6)</div>
							<div class="flex items-center gap-2 text-slate-300">
								<span class="inline-block h-2.5 w-2.5 rounded-full bg-cyan-400"></span> Southbound Directive Spine (L1→L6)
							</div>
							<div class="flex items-center gap-2 text-slate-300">
								<span class="inline-block h-2.5 w-2.5 rounded-full bg-amber-400"></span> Northbound Telemetry &amp; Gate Spine (L6→L1)
							</div>
							<div class="flex items-center gap-2 text-slate-300">
								<span class="inline-block h-2.5 w-2.5 rounded-full bg-emerald-400"></span> Wiki-Skill Nodes (Beta α/β) ·
								<span class="inline-block h-2.5 w-2.5 rounded-full bg-sky-400"></span> Connectome Slices
							</div>
						</div>

						<!-- Sensorium Proprio/Extero Overlay -->
						{#if v07Cockpit?.sensorium}
							<div class="pointer-events-none absolute top-3 right-3 rounded-xl border border-slate-800/90 bg-slate-950/85 p-2.5 text-[11px] text-right space-y-0.5">
								<div class="font-bold text-cyan-300">Sensorium &amp; Digital Twin</div>
								<div class="text-slate-300">
									CPU Load: <strong class="text-white">{(v07Cockpit.sensorium.cpu_load_1m ?? 0).toFixed(2)}</strong> ·
									RAM: <strong class="text-white">{(v07Cockpit.sensorium.mem_used_pct ?? 0).toFixed(0)}%</strong>
								</div>
								<div class="text-slate-300">
									4-Bit GPU Gate: <strong class={v07Cockpit.sensorium.gpu_gate_up ? 'text-emerald-300' : 'text-rose-300'}>{v07Cockpit.sensorium.gpu_gate_up ? 'ONLINE (<2ms)' : 'OFFLINE'}</strong>
								</div>
								<div class="text-slate-300">
									Circadian P(Human): <strong class="text-amber-300">{((v07Cockpit.sensorium.human_prob_active_now ?? 0.5) * 100).toFixed(0)}%</strong> ·
									Replicas: <strong class="text-violet-300">{v07Cockpit.sensorium.reality_replicas?.length ?? 1}</strong>
								</div>
							</div>
						{/if}
					</div>

					<!-- Selected 3D Element Inspector + Pyramidal Call Distribution Bar + Wiki-Skills -->
					<div class="max-h-[260px] overflow-y-auto border-t border-slate-800 bg-slate-950 p-3.5 space-y-3 text-xs">
						<!-- Pyramidal Call Share & Cache Hit Strip -->
						<div class="grid grid-cols-6 gap-1.5">
							{#each v07Cockpit?.layer_pyramid || [] as lp}
								<button
									onclick={() => {
										const lStat = engine?.layers?.find((l) => l.code === lp.layer_code);
										selected3DItem = { kind: 'layer', data: { ...lp, layerStatus: lStat } };
									}}
									class="rounded-lg border border-slate-800 bg-slate-900/90 p-2 text-left transition hover:border-violet-500/60"
								>
									<div class="flex items-center justify-between font-bold text-white">
										<span>{lp.layer_code}</span>
										<span class="text-[10px] text-violet-300">
											{lp.layer_code === 'L5' ? 'GPU' : `${(lp.share_pct || 0).toFixed(0)}%`}
										</span>
									</div>
									<div class="mt-0.5 text-[11px] text-slate-300">{lp.traces} runs</div>
									<div class="text-[10px] text-emerald-300">Cache: {(lp.cache_hit_pct || 0).toFixed(0)}%</div>
								</button>
							{/each}
						</div>

						<!-- Selected 3D Deck / Connectome Node Detail Card -->
						{#if selected3DItem}
							<div class="rounded-xl border border-violet-500/40 bg-slate-900/90 p-3 space-y-1.5">
								{#if selected3DItem.kind === 'layer'}
									<div class="flex items-center justify-between">
										<div class="font-bold text-white">
											Selected 3D Deck: <span class="text-violet-300">{selected3DItem.data.layer_code}</span>
											{#if selected3DItem.data.layerStatus}
												— {selected3DItem.data.layerStatus.name}
											{/if}
										</div>
										<div class="flex items-center gap-2">
											<span class="text-emerald-300 font-mono">
												KV Hit: {(selected3DItem.data.cache_hit_pct || 0).toFixed(1)}%
											</span>
											{#if selected3DItem.data.layerStatus}
												<button
													onclick={() => stepSingleLayer(selected3DItem.data.layerStatus.layer_id)}
													class="rounded bg-violet-600 px-2.5 py-0.5 font-bold text-white hover:bg-violet-500"
												>
													Step {selected3DItem.data.layer_code}
												</button>
											{/if}
										</div>
									</div>
									{#if selected3DItem.data.layerStatus}
										<div class="text-slate-200">{selected3DItem.data.layerStatus.summary}</div>
										{#if selected3DItem.data.layerStatus.active_directive}
											<div class="text-cyan-300 text-[11px]">
												<strong>Southbound:</strong> {selected3DItem.data.layerStatus.active_directive}
											</div>
										{/if}
									{/if}
								{:else}
									<div class="flex items-center justify-between">
										<div class="font-bold text-white">
											Connectome {selected3DItem.data.kind}: <span class="text-emerald-300">{selected3DItem.data.path || selected3DItem.data.id}</span>
										</div>
										<span class="rounded bg-slate-800 px-2 py-0.5 font-mono text-[10px] text-amber-300">
											VOI={(selected3DItem.data.voi || 0).toFixed(2)} · {selected3DItem.data.state}
										</span>
									</div>
									<div class="text-slate-300">{selected3DItem.data.summary}</div>
								{/if}
							</div>
						{/if}

						<!-- Active Wiki-Skills (Beta α/β Posteriors) & Verified Benchmark Results -->
						<div class="grid grid-cols-2 gap-2">
							<div class="rounded-xl border border-slate-800 bg-slate-900/70 p-2.5">
								<div class="mb-1.5 font-bold text-emerald-300">
									⚡ Active C6 Wiki-Skills (Beta α/β Lifecycle)
								</div>
								<div class="max-h-24 space-y-1 overflow-y-auto">
									{#each (v07Cockpit?.wiki_skills?.skills || []).filter((s: any) => !s.retired).slice(0, 8) as sk}
										<div class="flex items-center justify-between rounded bg-slate-950/80 px-2 py-1 text-[11px]">
											<span class="truncate text-slate-200" title={sk.wiki_path}>{sk.skill_id}</span>
											<span class="shrink-0 font-mono text-emerald-300">
												{((sk.alpha / Math.max(1, sk.alpha + sk.beta)) * 100).toFixed(0)}% ({sk.alpha}/{sk.beta})
											</span>
										</div>
									{/each}
								</div>
							</div>
							<div class="rounded-xl border border-slate-800 bg-slate-900/70 p-2.5">
								<div class="mb-1.5 font-bold text-violet-300">
									🏆 Verified Axis &amp; Ablation Artifacts ({v07Cockpit?.benchmark_results?.length ?? 0})
								</div>
								<div class="max-h-24 space-y-1 overflow-y-auto">
									{#each (v07Cockpit?.benchmark_results || []).slice(-8).reverse() as br}
										<div class="flex items-center justify-between rounded bg-slate-950/80 px-2 py-1 text-[11px]">
											<span class="truncate text-slate-200" title={br.file}>{br.file}</span>
											<span class="shrink-0 font-bold {br.h1_accepted ? 'text-emerald-400' : 'text-amber-400'}">
												{br.h1_accepted ? 'H1 PASS' : br.decision || 'EVAL'}
											</span>
										</div>
									{/each}
								</div>
							</div>
						</div>
					</div>
				</div>
			{:else}

			<!-- 5 HRM Global Loops Strip (from Original ACE Architecture) -->
			{#if engine?.global_loops && engine.global_loops.length > 0}
				<div class="grid grid-cols-5 gap-2 border-b border-slate-800 bg-slate-950/90 px-4 py-2.5 text-xs">
					{#each engine.global_loops as gl}
						<div class="rounded-lg border border-slate-800 bg-slate-900/90 px-2.5 py-1.5" title={gl.last_summary}>
							<div class="flex items-center justify-between gap-1">
								<span class="font-bold text-white truncate">{gl.name}</span>
								<span class="h-2 w-2 shrink-0 rounded-full {gl.state === 'active' ? 'bg-violet-400 animate-ping' : gl.state === 'blocked' ? 'bg-rose-400' : 'bg-emerald-400'}"></span>
							</div>
							<div class="mt-0.5 flex items-center justify-between text-[11px] text-slate-300">
								<span>{gl.frequency}</span>
								{#if gl.run_count > 0}
									<span class="font-mono text-violet-300">{gl.run_count}×</span>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			{/if}

			<div class="flex-1 space-y-3 overflow-y-auto p-4">
				{#each engine?.layers || [] as layer (layer.layer_id)}
					{@const style = layerStyles[layer.layer_id] || layerStyles[1]}
					{@const isThinking = layer.state === 'thinking' || steppingLayer === layer.layer_id}
					<div
						class="rounded-xl border bg-slate-900/90 p-4 transition-all {isThinking
							? 'border-violet-400 ring-2 ring-violet-500/40 shadow-lg shadow-violet-500/15'
							: style.border}"
					>
						<!-- Layer Header -->
						<div class="flex items-center justify-between gap-2">
							<div class="flex items-center gap-3">
								<span class="rounded-lg border px-2.5 py-1 text-sm font-extrabold {style.badge}">
									{layer.code}
								</span>
								<div>
									<div class="flex items-center gap-2.5">
										<h3 class="text-base font-bold text-white">{layer.name}</h3>
										<span
											class="rounded-md px-2 py-0.5 text-xs font-bold uppercase {isThinking
												? 'bg-violet-500/30 text-violet-200 animate-pulse'
												: layer.state === 'error'
													? 'bg-rose-500/30 text-rose-200'
													: 'bg-slate-800 text-slate-300'}"
										>
											{isThinking ? '⚡ THINKING' : layer.state}
										</span>
										{#if layer.inner_loop_step && layer.inner_loop_step > 0}
											<span class="rounded-md bg-rose-500/30 border border-rose-400/50 px-2 py-0.5 text-xs font-bold text-rose-200">
												Pi Loop {layer.inner_loop_step}/{layer.inner_loop_max || 4}
											</span>
										{/if}
									</div>
									<p class="text-xs text-slate-300">{layer.role}</p>
								</div>
							</div>

							<div class="flex items-center gap-1.5">
								{#if layer.total_invocations > 0}
									<span class="mr-1 text-xs font-medium text-slate-300">
										{layer.total_invocations}× · {(layer.last_latency_ms / 1000).toFixed(1)}s
									</span>
								{/if}
								<button
									onclick={() => (expandedLayerDetails[layer.layer_id] = !expandedLayerDetails[layer.layer_id])}
									class="rounded-lg border border-slate-700 bg-slate-800 px-2.5 py-1 text-xs font-semibold text-slate-200 hover:bg-slate-700"
								>
									{expandedLayerDetails[layer.layer_id] ? 'Hide Flow' : 'Show Flow'}
								</button>
								<button
									onclick={() => {
										thoughtLayerFilter = layer.layer_id;
										const latest = thoughts.find((t) => t.layer_id === layer.layer_id);
										if (latest) selectedThought = latest;
										rightTab = 'thoughts';
									}}
									class="rounded-lg border border-slate-700 bg-slate-800 px-2.5 py-1 text-xs font-semibold text-slate-200 hover:border-violet-400 hover:text-white"
								>
									Traces
								</button>
								<button
									onclick={() =>
										(openDirectiveInput[layer.layer_id] = !openDirectiveInput[layer.layer_id])}
									class="rounded-lg border border-slate-700 bg-slate-800 px-2.5 py-1 text-xs font-semibold text-slate-200 hover:border-violet-400 hover:text-white"
								>
									Inject
								</button>
								<button
									onclick={() => stepSingleLayer(layer.layer_id)}
									disabled={isThinking}
									class="rounded-lg bg-violet-600 px-3 py-1 text-xs font-bold text-white hover:bg-violet-500 disabled:opacity-40"
								>
									{isThinking ? '...' : `Step ${layer.code}`}
								</button>
							</div>
						</div>

						<!-- Headline Summary (High-contrast 15px) -->
						<div class="mt-3 rounded-lg border border-slate-800 bg-slate-950/90 px-3.5 py-2.5 text-sm leading-relaxed text-white">
							<span class="font-bold {style.accent}">Summary:</span> {layer.summary}
						</div>

						<!-- Live Stream While Thinking -->
						{#if isThinking && layer.live_stream}
							<div class="mt-3 rounded-lg border border-violet-500/50 bg-violet-950/30 p-3">
								<div class="mb-1 text-xs font-bold uppercase tracking-wide text-violet-200">
									⚡ Live Thinking Stream
								</div>
								<div class="max-h-40 overflow-y-auto whitespace-pre-wrap text-sm leading-relaxed text-violet-100">
									{cleanLiveStream(layer.live_stream)}
								</div>
							</div>
						{/if}

						<!-- Action Executed Pill (if any) -->
						{#if layer.last_action}
							<div class="mt-2.5 rounded-lg border border-emerald-500/40 bg-emerald-950/25 px-3 py-2 text-xs leading-relaxed text-emerald-200">
								<strong class="text-emerald-300">⚡ Last Action:</strong> {layer.last_action}
							</div>
						{/if}

						<!-- Expandable Full Southbound / Northbound Signals (No Truncation!) -->
						{#if expandedLayerDetails[layer.layer_id] && (layer.active_directive || layer.northbound_signal)}
							<div class="mt-2.5 space-y-2 text-xs leading-relaxed">
								{#if layer.active_directive}
									<div class="rounded-lg border border-slate-800 bg-slate-950/70 px-3 py-2 text-slate-200">
										<span class="font-bold text-cyan-300">▼ Southbound Directive:</span>
										<div class="mt-1 space-y-1">
											{#each formatReadableBlocks(layer.active_directive) as line}
												<p>{line}</p>
											{/each}
										</div>
									</div>
								{/if}
								{#if layer.northbound_signal}
									<div class="rounded-lg border border-slate-800 bg-slate-950/70 px-3 py-2 text-slate-200">
										<span class="font-bold text-amber-300">▲ Northbound Signal:</span>
										<div class="mt-1 space-y-1">
											{#each formatReadableBlocks(layer.northbound_signal) as line}
												<p>{line}</p>
											{/each}
										</div>
									</div>
								{/if}
							</div>
						{/if}

						<!-- Optional Manual Directive Injection Bar -->
						{#if openDirectiveInput[layer.layer_id]}
							<div class="mt-3 flex gap-2 pt-1">
								<input
									type="text"
									bind:value={activeLayerStimulus[layer.layer_id]}
									placeholder="Inject custom directive into {layer.code}..."
									class="flex-1 rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-white focus:border-violet-500 focus:outline-none"
								/>
								<button
									onclick={() => stepSingleLayer(layer.layer_id)}
									class="rounded-lg bg-violet-600 px-3.5 py-2 text-xs font-bold text-white hover:bg-violet-500"
								>
									Run {layer.code}
								</button>
								<button
									onclick={() =>
										triggerCascade(
											layer.layer_id,
											7 - layer.layer_id,
											activeLayerStimulus[layer.layer_id] || ''
										)}
									class="rounded-lg border border-violet-500/50 bg-violet-950/60 px-3 py-2 text-xs font-bold text-violet-200 hover:bg-violet-900/60"
								>
									Cascade {layer.code}→L6
								</button>
							</div>
						{/if}
					</div>
				{/each}
			</div>
			{/if}
		</section>

		<!-- COLUMN 3 (4 cols): MULTI-TAB DEEP INSPECTOR -->
		<section class="col-span-4 flex flex-col bg-slate-950/90 overflow-hidden">
			<!-- Inspector Navigation Tabs -->
			<div class="flex flex-wrap border-b border-slate-800 bg-slate-950 px-2 pt-2 text-xs">
				{#each [
					{ id: 'explainer', label: '🔍 Explainer' },
					{ id: 'thoughts', label: `🧠 Thoughts (${thoughts.length})` },
					{ id: 'telem', label: `📊 Telem (${telemetrySpans.length})` },
					{ id: 'bus', label: `⇅ Bus (${busEvents.length})` },
					{ id: 'wiki', label: `📚 Wiki (${wikiPages.length})` },
					{ id: 'memory', label: `🗂️ FTS5 (${memories.length})` },
					{ id: 'sandbox', label: '🛡️ Sandbox' },
					{ id: 'telegram', label: '📱 Telegram' }
				] as tab}
					<button
						onclick={() => (rightTab = tab.id as any)}
						class="border-b-2 px-3 py-2.5 font-bold transition {rightTab === tab.id
							? 'border-violet-500 text-white'
							: 'border-transparent text-slate-400 hover:text-slate-200'}"
					>
						{tab.label}
					</button>
				{/each}
			</div>

			<!-- TAB 0: 6-LAYER VISUAL EXPLAINER (WHAT EACH LAYER SEES, DECIDES & DOES) -->
			{#if rightTab === 'explainer'}
				<div class="flex-1 overflow-y-auto p-4 space-y-4 text-xs">
					<div class="rounded-xl border border-violet-500/40 bg-violet-950/20 p-3.5 space-y-1.5">
						<div class="flex items-center justify-between">
							<span class="font-extrabold uppercase tracking-wider text-violet-300">
								🔍 Live 6-Layer Cognitive Explainer
							</span>
							<span class="rounded-md bg-violet-600/30 px-2 py-0.5 text-[11px] font-bold text-violet-200">
								256k Context · Prefix Cached
							</span>
						</div>
						<p class="text-slate-300 leading-relaxed">
							Follows how stimuli flow down the <strong>Southbound Directive Bus (L1→L6)</strong>, execute inside the <strong>bwrap Sandbox (L6 Pi Prosecutor)</strong>, and consolidate up the <strong>Northbound Telemetry Bus (L6→L1)</strong>.
						</p>
					</div>

					{#if engine}
						{#each engine.layers as layer (layer.layer_id)}
							{@const latestTrace = thoughts.find((t) => t.layer_id === layer.layer_id)}
							<div class="rounded-xl border border-slate-800 bg-slate-900/80 p-3.5 space-y-2.5">
								<div class="flex items-center justify-between border-b border-slate-800 pb-2">
									<div class="flex items-center gap-2">
										<span class="rounded-lg bg-violet-600 px-2 py-0.5 font-extrabold text-white">
											{layer.code}
										</span>
										<div>
											<div class="font-bold text-white">{layer.name}</div>
											<div class="text-[11px] text-slate-400">{layer.role}</div>
										</div>
									</div>
									<div class="text-right text-[11px] text-slate-400">
										<div>Runs: <strong class="text-white">{layer.total_invocations}</strong> · {(layer.last_latency_ms / 1000).toFixed(1)}s</div>
										{#if latestTrace}
											<div>Tokens: <span class="text-emerald-300">{latestTrace.prompt_tokens} in</span> → <span class="text-sky-300">{latestTrace.completion_tokens} out</span></div>
										{/if}
									</div>
								</div>

								<!-- 1. WHAT IT SEES -->
								<div class="rounded-lg border border-slate-800/90 bg-slate-950/80 p-2.5 space-y-1">
									<div class="font-bold uppercase tracking-wider text-sky-400 text-[10px]">
										1. 👀 What {layer.code} Sees (Stimulus & Context)
									</div>
									<div class="text-slate-200">
										{latestTrace?.trigger_summary || layer.active_directive || 'Waiting for stimulus...'}
									</div>
									<div class="text-[11px] text-slate-400">
										Context: <code>soul.md</code> + <code>goals.md</code> + <code>{layer.code}/GLOBAL</code> FTS5 Memories + Sandbox File Tree
									</div>
								</div>

								<!-- 2. WHAT IT DECIDES -->
								<div class="rounded-lg border border-slate-800/90 bg-slate-950/80 p-2.5 space-y-1">
									<div class="font-bold uppercase tracking-wider text-violet-400 text-[10px]">
										2. 🧠 What {layer.code} Thinks & Decides ({Math.round((layer.confidence || 0.9) * 100)}% conf)
									</div>
									<div class="font-bold text-white">{layer.summary}</div>
									{#if layer.last_thought}
										<div class="whitespace-pre-line text-slate-300 text-[11px] pt-0.5">
											{layer.last_thought}
										</div>
									{/if}
								</div>

								<!-- 3. WHAT IT DOES & PASSES DOWN -->
								<div class="rounded-lg border border-slate-800/90 bg-slate-950/80 p-2.5 space-y-1.5">
									<div class="font-bold uppercase tracking-wider text-emerald-400 text-[10px]">
										3. ⚡ What {layer.code} Executes & Emits
									</div>
									{#if latestTrace?.action_type && latestTrace.action_type !== 'none'}
										<div class="rounded bg-emerald-950/50 border border-emerald-500/30 px-2 py-1 font-mono text-[11px] text-emerald-200">
											Tool: <strong>{latestTrace.action_type}</strong>({latestTrace.action_target})
											{#if latestTrace.action_result}
												<div class="mt-1 text-slate-300 line-clamp-3">{latestTrace.action_result}</div>
											{/if}
										</div>
									{/if}
									{#if layer.active_directive && layer.layer_id < 6}
										<div class="text-amber-200 text-[11px]">
											<strong>↓ To L{layer.layer_id + 1}:</strong> {layer.active_directive}
										</div>
									{/if}
									{#if layer.northbound_signal && layer.layer_id > 1}
										<div class="text-sky-200 text-[11px]">
											<strong>↑ To L{layer.layer_id - 1}:</strong> {layer.northbound_signal}
										</div>
									{/if}
								</div>
							</div>
						{/each}
					{/if}
				</div>
			{/if}

			<!-- TAB 1: THOUGHT TRACE & RAW PROMPT INSPECTOR -->
			{#if rightTab === 'thoughts'}
				<div class="flex flex-1 flex-col overflow-hidden">
					<!-- Layer Filter Bar + Follow Live Toggle -->
					<div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-800 bg-slate-900/60 px-3.5 py-2.5 text-xs">
						<div class="flex flex-wrap items-center gap-1">
							{#each [
								{ id: -1, label: 'ALL' },
								{ id: 0, label: 'CHAT' },
								{ id: 1, label: 'L1' },
								{ id: 2, label: 'L2' },
								{ id: 3, label: 'L3' },
								{ id: 4, label: 'L4' },
								{ id: 5, label: 'L5' },
								{ id: 6, label: 'L6' }
							] as lf}
								<button
									onclick={() => {
										thoughtLayerFilter = lf.id;
										const first =
											lf.id === -1 ? thoughts[0] : thoughts.find((t) => t.layer_id === lf.id);
										if (first) selectedThought = first;
									}}
									class="rounded-md px-2.5 py-1 font-bold {thoughtLayerFilter === lf.id
										? 'bg-violet-600 text-white'
										: 'bg-slate-800 text-slate-300 hover:text-white'}"
								>
									{lf.label}
								</button>
							{/each}
						</div>

						<label class="flex cursor-pointer items-center gap-1.5 text-xs font-semibold text-slate-300">
							<input type="checkbox" bind:checked={followLiveThoughts} class="rounded accent-violet-500" />
							<span>Follow Live</span>
						</label>
					</div>

					<!-- Trace Selector Strip (compact horizontal pills so Detail View gets maximum vertical space!) -->
					<div class="flex items-center gap-1.5 overflow-x-auto border-b border-slate-800 bg-slate-950 px-3.5 py-2">
						{#each filteredThoughts.slice(0, 15) as t (t.id)}
							<button
								onclick={() => {
									selectedThought = t;
									followLiveThoughts = false;
								}}
								class="shrink-0 rounded-lg border px-2.5 py-1 text-xs font-semibold transition {selectedThought?.id === t.id
									? 'border-violet-500 bg-violet-600/25 text-white'
									: 'border-slate-800 bg-slate-900 text-slate-300 hover:border-slate-700 hover:text-white'}"
							>
								<span class="font-bold text-violet-300">{t.layer_code}</span> · {(t.latency_ms / 1000).toFixed(1)}s
							</button>
						{/each}
					</div>

					<!-- Selected Thought Detail (Full Height, Easy to Read!) -->
					{#if selectedThought}
						<div class="flex-1 overflow-y-auto p-4 space-y-4 text-sm leading-relaxed">
							<div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-800 pb-3">
								<div class="flex items-center gap-2.5">
									<span class="rounded-lg bg-violet-600 px-2.5 py-1 text-xs font-extrabold text-white">
										{selectedThought.layer_code}
									</span>
									<div>
										<div class="font-bold text-white">{selectedThought.state_summary}</div>
										<div class="text-xs text-slate-400">
											Model: {selectedThought.model} · {(selectedThought.latency_ms / 1000).toFixed(1)}s · Tokens: {selectedThought.prompt_tokens}→{selectedThought.completion_tokens}
										</div>
									</div>
								</div>
								<button
									onclick={() => (showRawPrompts = !showRawPrompts)}
									class="rounded-lg border border-slate-700 bg-slate-800 px-3 py-1.5 text-xs font-bold text-slate-200 hover:border-violet-500 hover:text-white"
								>
									{showRawPrompts ? 'Structured View' : 'Raw Prompts & JSON'}
								</button>
							</div>

							{#if !showRawPrompts}
								<!-- Formatted Bullet-Point Internal Monologue -->
								<div class="rounded-xl border border-slate-800 bg-slate-900/90 p-4">
									<div class="mb-2 text-xs font-bold uppercase tracking-wider text-violet-300">
										Internal Monologue (Thought)
									</div>
									<div class="space-y-2 text-sm leading-relaxed text-slate-100">
										{#each formatReadableBlocks(selectedThought.thought || selectedThought.raw_reasoning) as block}
											<p>{block}</p>
										{/each}
									</div>
								</div>

								{#if selectedThought.southbound}
									<div class="rounded-xl border border-cyan-500/30 bg-cyan-950/15 p-4">
										<div class="mb-1.5 text-xs font-bold uppercase tracking-wider text-cyan-300">
											▼ Southbound Directive Emitted
										</div>
										<div class="space-y-1.5 text-sm leading-relaxed text-slate-100">
											{#each formatReadableBlocks(selectedThought.southbound) as block}
												<p>{block}</p>
											{/each}
										</div>
									</div>
								{/if}

								{#if selectedThought.northbound}
									<div class="rounded-xl border border-amber-500/30 bg-amber-950/15 p-4">
										<div class="mb-1.5 text-xs font-bold uppercase tracking-wider text-amber-300">
											▲ Northbound Telemetry / Signal
										</div>
										<div class="space-y-1.5 text-sm leading-relaxed text-slate-100">
											{#each formatReadableBlocks(selectedThought.northbound) as block}
												<p>{block}</p>
											{/each}
										</div>
									</div>
								{/if}

								{#if selectedThought.action_type && selectedThought.action_type !== 'none'}
									<div class="rounded-xl border border-emerald-500/40 bg-emerald-950/20 p-4">
										<div class="mb-1.5 text-xs font-bold uppercase tracking-wider text-emerald-300">
											⚡ Actuator: {selectedThought.action_type} → {selectedThought.action_target}
										</div>
										{#if selectedThought.action_result}
											<pre class="mt-2 overflow-x-auto whitespace-pre-wrap rounded-lg bg-slate-950 p-3 font-mono text-xs leading-relaxed text-emerald-200">{selectedThought.action_result}</pre>
										{/if}
									</div>
								{/if}
							{:else}
								<div class="space-y-3 text-xs">
									<div>
										<div class="mb-1 font-bold text-amber-300">=== SYSTEM PROMPT ===</div>
										<pre class="max-h-56 overflow-y-auto whitespace-pre-wrap rounded-lg border border-slate-800 bg-slate-900 p-3 font-mono leading-relaxed text-slate-200">{selectedThought.system_prompt}</pre>
									</div>
									<div>
										<div class="mb-1 font-bold text-cyan-300">=== USER STIMULUS PROMPT ===</div>
										<pre class="max-h-56 overflow-y-auto whitespace-pre-wrap rounded-lg border border-slate-800 bg-slate-900 p-3 font-mono leading-relaxed text-slate-200">{selectedThought.user_prompt}</pre>
									</div>
									<div>
										<div class="mb-1 font-bold text-emerald-300">=== RAW MODEL OUTPUT ===</div>
										<pre class="max-h-56 overflow-y-auto whitespace-pre-wrap rounded-lg border border-slate-800 bg-slate-900 p-3 font-mono leading-relaxed text-slate-200">{selectedThought.raw_response}</pre>
									</div>
								</div>
							{/if}
						</div>
					{:else}
						<div class="p-6 text-center text-sm text-slate-400">
							No thought traces recorded yet. Click "Step L1" or "Cascade L1→L6" to watch ACE think.
						</div>
					{/if}
				</div>

			<!-- TAB 1B: OPENTELEMETRY SPANS, METRICS & HRM GLOBAL LOOPS -->
			{:else if rightTab === 'telem'}
				<div class="flex-1 overflow-y-auto p-4 space-y-4 text-sm">
					<!-- Metrics Summary Grid -->
					<div class="grid grid-cols-3 gap-2.5">
						<div class="rounded-xl border border-slate-800 bg-slate-900/90 p-3">
							<div class="text-xs text-slate-400">Prompt / Out Tokens</div>
							<div class="mt-1 text-base font-extrabold text-white">
								{(engine?.metrics?.total_prompt_tokens || 0).toLocaleString()} / {(engine?.metrics?.total_completion_tokens || 0).toLocaleString()}
							</div>
						</div>
						<div class="rounded-xl border border-slate-800 bg-slate-900/90 p-3">
							<div class="text-xs text-slate-400">Pi Tool Calls / Spans</div>
							<div class="mt-1 text-base font-extrabold text-violet-300">
								{engine?.metrics?.total_tool_calls || 0} tools · {telemetrySpans.length} spans
							</div>
						</div>
						<div class="rounded-xl border border-slate-800 bg-slate-900/90 p-3">
							<div class="text-xs text-slate-400">Safety Monitor</div>
							<div class="mt-1 text-base font-extrabold text-emerald-300">
								{engine?.metrics?.safety_checks_passed || 0} pass · {engine?.metrics?.safety_interventions || 0} blocked
							</div>
						</div>
					</div>

					<!-- Average Layer Latency Bar -->
					{#if engine?.metrics?.avg_layer_latency_ms}
						<div class="rounded-xl border border-slate-800 bg-slate-900/80 p-3.5">
							<div class="mb-2 text-xs font-bold uppercase tracking-wider text-slate-300">Avg Latency By Layer</div>
							<div class="grid grid-cols-6 gap-2 text-center text-xs">
								{#each ['L1', 'L2', 'L3', 'L4', 'L5', 'L6'] as code}
									<div class="rounded-lg border border-slate-800 bg-slate-950 p-2">
										<div class="font-bold text-violet-300">{code}</div>
										<div class="mt-0.5 font-mono text-white">
											{(((engine.metrics.avg_layer_latency_ms[code] || 0) / 1000)).toFixed(1)}s
										</div>
									</div>
								{/each}
							</div>
						</div>
					{/if}

					<!-- Live OpenTelemetry Spans -->
					<div class="space-y-2.5">
						<div class="text-xs font-bold uppercase tracking-wider text-slate-300">OpenTelemetry Execution Spans ({telemetrySpans.length})</div>
						{#each telemetrySpans as sp (sp.span_id)}
							<div class="rounded-xl border border-slate-800 bg-slate-900/90 p-3 text-xs">
								<div class="flex items-center justify-between gap-2">
									<div class="flex items-center gap-2">
										<span class="rounded bg-violet-600/30 border border-violet-500/40 px-2 py-0.5 font-bold text-violet-200">{sp.layer_code}</span>
										<span class="font-mono font-bold text-cyan-300">{sp.operation}</span>
										<span class="rounded px-1.5 py-0.5 font-bold uppercase {sp.status === 'ok' ? 'bg-emerald-500/20 text-emerald-300' : 'bg-rose-500/20 text-rose-300'}">{sp.status}</span>
									</div>
									<span class="font-mono text-slate-400">{sp.duration_ms}ms · {sp.trace_id}</span>
								</div>
								<div class="mt-1.5 text-slate-200 leading-relaxed">{sp.detail}</div>
							</div>
						{/each}
					</div>
				</div>

			<!-- TAB 2: NORTHBOUND / SOUTHBOUND BUS STREAM -->
			{:else if rightTab === 'bus'}
				<div class="flex-1 space-y-3 overflow-y-auto p-4">
					{#each busEvents as b (b.id)}
						{@const isSouth = b.direction.toLowerCase() === 'southbound'}
						<div class="rounded-xl border border-slate-800 bg-slate-900/80 p-3.5 text-sm">
							<div class="flex items-center justify-between text-xs">
								<span
									class="rounded-md px-2 py-0.5 font-bold {isSouth
										? 'bg-cyan-500/20 text-cyan-300'
										: 'bg-amber-500/20 text-amber-300'}"
								>
									{isSouth ? '▼ SOUTHBOUND' : '▲ NORTHBOUND'} · L{b.from_layer} → L{b.to_layer}
								</span>
								<span class="font-mono text-slate-400">{b.topic}</span>
							</div>
							<div class="mt-2 space-y-1 text-slate-100 leading-relaxed">
								{#each formatReadableBlocks(b.payload || b.summary) as line}
									<p>{line}</p>
								{/each}
							</div>
						</div>
					{/each}
				</div>

			<!-- TAB 3: KARPATHY LLM WIKI -->
			{:else if rightTab === 'wiki'}
				<div class="flex flex-1 flex-col overflow-hidden p-4 gap-3">
					<div class="flex flex-wrap gap-2">
						{#each wikiPages as page}
							<button
								onclick={() => loadWikiPage(page.path)}
								class="rounded-lg border px-3 py-1.5 text-xs font-bold {selectedWikiPath === page.path
									? 'border-violet-500 bg-violet-600/25 text-white'
									: 'border-slate-800 bg-slate-900 text-slate-300 hover:border-slate-700 hover:text-white'}"
							>
								📄 {page.path}
							</button>
						{/each}
					</div>

					<div class="flex items-center gap-2">
						<input
							type="text"
							bind:value={newWikiPath}
							placeholder="Or new page path (e.g. notes/goals.md)..."
							class="flex-1 rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-white"
						/>
						<button
							onclick={saveWikiPage}
							disabled={wikiSaving}
							class="rounded-lg bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-500"
						>
							{wikiSaving ? 'Saving...' : 'Save Page'}
						</button>
					</div>

					<textarea
						bind:value={wikiContent}
						class="flex-1 w-full rounded-xl border border-slate-800 bg-slate-900 p-4 font-mono text-sm leading-relaxed text-slate-100 focus:border-violet-500 focus:outline-none"
					></textarea>
				</div>

			<!-- TAB 4: SQLITE FTS5 EPISODIC MEMORY -->
			{:else if rightTab === 'memory'}
				<div class="flex flex-1 flex-col overflow-hidden p-4 gap-3.5">
					<div class="flex gap-2">
						<input
							type="text"
							bind:value={memoryQuery}
							oninput={searchMemories}
							placeholder="Search SQLite FTS5 memories..."
							class="flex-1 rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-white"
						/>
						<button
							onclick={searchMemories}
							class="rounded-lg bg-slate-800 px-3.5 py-2 text-xs font-bold text-white hover:bg-slate-700"
						>
							Search
						</button>
					</div>

					<div class="rounded-xl border border-slate-800 bg-slate-900/60 p-3.5 space-y-2.5">
						<div class="text-xs font-bold uppercase text-slate-300">Record Episodic Memory</div>
						<input
							type="text"
							bind:value={newMemTitle}
							placeholder="Memory title..."
							class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-1.5 text-sm text-white"
						/>
						<textarea
							bind:value={newMemContent}
							rows="2"
							placeholder="Memory content..."
							class="w-full rounded-lg border border-slate-700 bg-slate-950 p-2.5 text-sm text-white"
						></textarea>
						<div class="flex justify-between items-center gap-2">
							<input
								type="text"
								bind:value={newMemTags}
								placeholder="tags (comma-separated)"
								class="flex-1 rounded-lg border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-200"
							/>
							<button
								onclick={addMemory}
								class="rounded-lg bg-violet-600 px-3.5 py-1.5 text-xs font-bold text-white hover:bg-violet-500"
							>
								+ Save to FTS5
							</button>
						</div>
					</div>

					<div class="flex-1 space-y-2.5 overflow-y-auto">
						{#each memories as m (m.id)}
							<div class="rounded-xl border border-slate-800 bg-slate-900/90 p-3.5 text-sm leading-relaxed">
								<div class="flex items-center justify-between text-xs text-slate-400">
									<span class="font-bold text-violet-300">[{m.layer_code} · {m.tier || 'medium'} · {m.category}]</span>
									<span>{m.tags}</span>
								</div>
								<div class="mt-1 font-bold text-white">{m.title}</div>
								<p class="mt-1 text-slate-200 whitespace-pre-wrap">{m.content}</p>
							</div>
						{/each}
					</div>
				</div>

			<!-- TAB 5: HOST-PROOF SANDBOX INSPECTOR -->
			{:else if rightTab === 'sandbox'}
				<div class="flex flex-1 flex-col overflow-y-auto p-4 space-y-3.5 text-sm">
					<div class="rounded-xl border border-emerald-500/40 bg-emerald-950/20 p-4">
						<div class="flex items-center justify-between">
							<h3 class="font-bold text-emerald-300">3-Layer Host Isolation Verification</h3>
							<button
								onclick={reverifySandbox}
								class="rounded-lg border border-emerald-500/40 bg-emerald-900/50 px-3 py-1 text-xs font-bold text-emerald-100 hover:bg-emerald-800/60"
							>
								Re-Verify Now
							</button>
						</div>
						<div class="mt-2.5 grid grid-cols-2 gap-2 text-xs text-slate-200">
							<div>✅ Go <code>os.Root</code> openat2: <strong>{engine?.sandbox?.os_root_active ? 'ACTIVE' : 'NO'}</strong></div>
							<div>✅ <code>../</code> Escape Blocked: <strong>{engine?.sandbox?.traversal_blocked ? 'BLOCKED' : 'NO'}</strong></div>
							<div>✅ Bubblewrap: <strong>{engine?.sandbox?.bwrap_version || 'N/A'}</strong></div>
							<div>✅ Host <code>/home</code> Unmounted: <strong>{engine?.sandbox?.home_hidden ? 'HIDDEN' : 'VISIBLE'}</strong></div>
							<div>✅ Network Unshared: <strong>{engine?.sandbox?.network_unshared ? 'ISOLATED' : 'SHARED'}</strong></div>
						</div>
						<p class="mt-2 font-mono text-xs text-slate-400">Jail Root: {engine?.sandbox?.sandbox_dir}</p>
					</div>

					<!-- Interactive bwrap Shell Tester -->
					<div class="rounded-xl border border-slate-800 bg-slate-900/80 p-3.5 space-y-2.5">
						<div class="font-bold text-white text-xs uppercase tracking-wide">Run Command Inside Isolated <code>bwrap --unshare-all</code></div>
						<div class="flex gap-2">
							<input
								type="text"
								bind:value={sandboxCmd}
								class="flex-1 rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs text-white"
							/>
							<button
								onclick={runSandboxCommand}
								disabled={sandboxRunning}
								class="rounded-lg bg-violet-600 px-4 py-2 text-xs font-bold text-white hover:bg-violet-500 disabled:opacity-40"
							>
								{sandboxRunning ? 'Running...' : 'Exec'}
							</button>
						</div>
					</div>

					<!-- Workspace Files -->
					<div class="rounded-xl border border-slate-800 bg-slate-900/70 p-3.5">
						<div class="mb-2 text-xs font-bold uppercase tracking-wide text-slate-300">Sandboxed <code>/workspace</code> Files ({workspaceFiles.length})</div>
						<div class="flex flex-wrap gap-2">
							{#each workspaceFiles as wf}
								<button
									onclick={() => readWorkspaceFile(wf.path)}
									class="rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1.5 font-mono text-xs text-slate-200 hover:border-violet-500 hover:text-white"
								>
									{wf.path} ({wf.size_bytes}B)
								</button>
							{/each}
						</div>
						{#if selectedWorkspaceFile}
							<pre class="mt-3 max-h-40 overflow-y-auto rounded-lg bg-slate-950 p-3 font-mono text-xs leading-relaxed text-emerald-200 whitespace-pre-wrap">{workspaceFileContent}</pre>
						{/if}
					</div>

					<!-- Recent bwrap Executions -->
					<div class="space-y-2.5">
						<div class="text-xs font-bold uppercase tracking-wide text-slate-300">Recent Sandbox Executions</div>
						{#each recentExecs as ex}
							<div class="rounded-xl border border-slate-800 bg-slate-950 p-3 font-mono text-xs">
								<div class="flex justify-between text-slate-400">
									<span class="font-bold text-cyan-300">$ {ex.command}</span>
									<span>exit={ex.exit_code} · {ex.duration_ms}ms</span>
								</div>
								{#if ex.stdout}
									<pre class="mt-2 whitespace-pre-wrap leading-relaxed text-slate-200">{ex.stdout}</pre>
								{/if}
								{#if ex.stderr}
									<pre class="mt-2 whitespace-pre-wrap leading-relaxed text-rose-300">{ex.stderr}</pre>
								{/if}
							</div>
						{/each}
					</div>
				</div>

			<!-- TAB 6: TELEGRAM BOT CHANNEL ADAPTER -->
			{:else if rightTab === 'telegram'}
				<div class="flex flex-1 flex-col overflow-y-auto p-4 space-y-4 text-sm">
					<div class="rounded-xl border {telegram?.connected ? 'border-sky-500/40 bg-sky-950/20' : 'border-slate-800 bg-slate-900/70'} p-4">
						<div class="flex items-center justify-between">
							<div>
								<h3 class="font-bold text-white">📱 Telegram Bot Channel Adapter</h3>
								<p class="mt-0.5 text-xs text-slate-300">
									Long-polling bridge (`getUpdates`) — no public webhook or port-forwarding required.
								</p>
							</div>
							<span class="rounded-full px-2.5 py-1 text-xs font-bold {telegram?.connected ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40' : 'bg-slate-800 text-slate-400'}">
								{telegram?.connected ? `CONNECTED (@${telegram.bot_username})` : 'DISCONNECTED'}
							</span>
						</div>

						{#if telegram}
							<div class="mt-3 grid grid-cols-3 gap-2 text-xs">
								<div class="rounded-lg bg-slate-950/80 p-2.5">
									<div class="text-slate-400">Token</div>
									<div class="mt-0.5 font-mono text-white">{telegram.masked_token || 'Not set'}</div>
								</div>
								<div class="rounded-lg bg-slate-950/80 p-2.5">
									<div class="text-slate-400">Last Chat ID</div>
									<div class="mt-0.5 font-mono text-cyan-300">{telegram.last_chat_id || 'Awaiting /start'}</div>
								</div>
								<div class="rounded-lg bg-slate-950/80 p-2.5">
									<div class="text-slate-400">Messages In / Out</div>
									<div class="mt-0.5 font-mono text-emerald-300">{telegram.messages_in} in · {telegram.messages_out} out</div>
								</div>
							</div>
							{#if telegram.last_error}
								<div class="mt-2.5 rounded-lg border border-rose-500/40 bg-rose-950/40 p-2.5 text-xs text-rose-200">
									⚠️ {telegram.last_error}
								</div>
							{/if}
						{/if}
					</div>

					<!-- Setup Form -->
					<div class="rounded-xl border border-slate-800 bg-slate-900/80 p-4 space-y-3">
						<div class="text-xs font-bold uppercase tracking-wider text-slate-300">Configure BotFather Token & Security</div>
						<div>
							<label class="block text-xs font-semibold text-slate-300 mb-1">
								Bot Token (from <code>@BotFather</code> on Telegram, or set <code>ACE_TELEGRAM_TOKEN</code>)
							</label>
							<input
								type="password"
								bind:value={tgTokenInput}
								placeholder={telegram?.masked_token ? `Saved (${telegram.masked_token}) — leave blank to keep` : '123456789:ABCdefGHIjklMNOpqrSTUvwxYZ...'}
								class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs text-white focus:border-sky-500 focus:outline-none"
							/>
						</div>
						<div>
							<label class="block text-xs font-semibold text-slate-300 mb-1">
								Allowed Chat IDs (comma-separated, or leave blank while sending <code>/start</code> to discover your ID)
							</label>
							<input
								type="text"
								bind:value={tgAllowedInput}
								placeholder="e.g. 123456789"
								class="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs text-white focus:border-sky-500 focus:outline-none"
							/>
						</div>
						<label class="flex items-center gap-2 text-xs text-slate-200 cursor-pointer">
							<input type="checkbox" bind:checked={tgForwardProactive} class="rounded border-slate-700 bg-slate-900" />
							<span>Automatically forward <strong>⚡ L6 Proactive</strong> sandbox/wiki updates to Telegram</span>
						</label>

						<div class="flex flex-wrap gap-2 pt-1">
							<button
								onclick={() => saveTelegramConfig(true)}
								disabled={tgSaving}
								class="rounded-lg bg-sky-600 px-4 py-2 text-xs font-bold text-white hover:bg-sky-500 disabled:opacity-50"
							>
								{tgSaving ? 'Connecting...' : 'Save & Connect Bot'}
							</button>
							{#if telegram?.connected}
								<button
									onclick={sendTelegramTestPing}
									class="rounded-lg border border-emerald-500/50 bg-emerald-950/60 px-3.5 py-2 text-xs font-bold text-emerald-200 hover:bg-emerald-900/60"
								>
									Send Test Ping
								</button>
								<button
									onclick={() => saveTelegramConfig(false)}
									class="rounded-lg border border-rose-500/50 bg-rose-950/60 px-3.5 py-2 text-xs font-bold text-rose-200 hover:bg-rose-900/60"
								>
									Disconnect
								</button>
							{/if}
						</div>

						{#if tgFeedback}
							<div class="rounded-lg bg-slate-950 px-3 py-2 text-xs text-sky-300 font-semibold">
								{tgFeedback}
							</div>
						{/if}
					</div>

					<!-- Telegram Commands Reference -->
					<div class="rounded-xl border border-slate-800 bg-slate-900/60 p-4 space-y-2 text-xs text-slate-300">
						<div class="font-bold uppercase tracking-wider text-white">Supported Telegram Commands</div>
						<ul class="space-y-1.5 font-mono">
							<li><strong class="text-sky-300">&lt;any message&gt;</strong> — Chats with ACE Fast Chat + triggers L1→L6 cascade</li>
							<li><strong class="text-sky-300">/start</strong> — Displays your Telegram Chat ID & command menu</li>
							<li><strong class="text-sky-300">/status</strong> — Shows live 6-layer cognitive state & sandbox health</li>
							<li><strong class="text-sky-300">/goals</strong> — Reads active mission from <code>/wiki/goals.md</code></li>
							<li><strong class="text-sky-300">/cycle &lt;directive&gt;</strong> — Triggers a full L1→L6 cognitive cascade</li>
							<li><strong class="text-sky-300">/mode &lt;reactive|autonomous|manual|paused&gt;</strong> — Switches mode</li>
							<li><strong class="text-sky-300">/exec &lt;cmd&gt;</strong> — Executes command inside isolated <code>bwrap</code> sandbox</li>
							<li><strong class="text-sky-300">/tickets</strong> — Lists Pi tickets in <code>/workspace/.tickets/tickets.json</code></li>
							<li><strong class="text-sky-300">/wiki [path]</strong> — Lists or reads pages in ACE's LLM Wiki</li>
						</ul>
					</div>
				</div>
			{/if}
		</section>
	</div>
</div>
