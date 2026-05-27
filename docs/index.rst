:orphan:

.. meta::
   :description: Kubeflow Trainer — Kubernetes-native distributed AI training platform
   :keywords: kubeflow, trainer, distributed training, kubernetes, pytorch, llm, fine-tuning

.. raw:: html

   <div class="landing-page">

   <section class="hero">
   <div class="hero-bg-pattern"></div>
   <div class="hero-content">
   <div class="hero-badge">Open Source &middot; CNCF Project</div>
   <h1 class="hero-title">Kubeflow Trainer</h1>
   <p class="hero-tagline">The Kubernetes-native platform for distributed AI training and LLM fine-tuning at any scale.</p>
   <div class="hero-actions">
   <a href="getting-started/index.html" class="btn btn-primary">Get Started <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M5 12h14M12 5l7 7-7 7"/></svg></a>
   <a href="https://github.com/kubeflow/trainer" class="btn btn-secondary">GitHub</a>
   </div>
   <div class="hero-sub">Deploy anywhere you run Kubernetes &mdash; or train locally with Docker.</div>
   </div>
   </section>

   <section class="stats-bar">
   <div class="stat"><span class="stat-value">2.1k+</span><span class="stat-label">GitHub Stars</span></div>
   <div class="stat-divider"></div>
   <div class="stat"><span class="stat-value">8+</span><span class="stat-label">ML Frameworks</span></div>
   <div class="stat-divider"></div>
   <div class="stat"><span class="stat-value">250+</span><span class="stat-label">Contributors</span></div>
   <div class="stat-divider"></div>
   <div class="stat"><span class="stat-value">960+</span><span class="stat-label">Forks</span></div>
   </section>

   <section class="what-is">
   <h2 class="section-title">What is Kubeflow Trainer?</h2>
   <p class="section-desc">Kubeflow Trainer is a Kubernetes-native platform for distributed AI model training and LLM fine-tuning. It provides a single TrainJob CRD and a unified Python SDK across PyTorch, JAX, DeepSpeed, MLX, HuggingFace, Megatron, and XGBoost. Train locally with Docker or scale to multi-node GPU clusters on any Kubernetes environment &mdash; without changing your code. It features distributed data caching with Apache Arrow and Apache DataFusion for zero-copy tensor streaming directly to GPU nodes.</p>
   </section>

   <section class="features">
   <h2 class="section-title">Why Trainer?</h2>
   <div class="features-grid">

   <div class="feature-card">
   <div class="feature-icon"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="var(--lp-blue)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="16" height="16" rx="2"/><path d="M4 12h16M12 4v16"/></svg></div>
   <h3>Multi-Framework</h3>
   <p>One API for PyTorch, JAX, DeepSpeed, MLX, HuggingFace, Megatron, XGBoost, and more. Swap frameworks without rewriting orchestration code.</p>
   </div>

   <div class="feature-card">
   <div class="feature-icon"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="var(--lp-blue)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="6" cy="12" r="3"/><circle cx="18" cy="6" r="3"/><circle cx="18" cy="18" r="3"/><path d="M9 12h3m0 0l3-4.5M12 12l3 4.5"/></svg></div>
   <h3>Distributed Training</h3>
   <p>Scale from a single GPU to multi-node clusters. Automatic setup of DDP, FSDP, parameter servers, and gang-scheduling across nodes.</p>
   </div>

   <div class="feature-card">
   <div class="feature-icon"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="var(--lp-blue)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg></div>
   <h3>Local &amp; Cloud</h3>
   <p>Develop and test locally with Docker or Podman, then deploy the same TrainJob to any Kubernetes cluster &mdash; zero code changes needed.</p>
   </div>

   <div class="feature-card">
   <div class="feature-icon"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="var(--lp-blue)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg></div>
   <h3>LLM Fine-Tuning</h3>
   <p>First-class support for LoRA, QLoRA, and full fine-tuning via TorchTune. Bring your own HuggingFace model and dataset URIs.</p>
   </div>

   <div class="feature-card">
   <div class="feature-icon"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="var(--lp-blue)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg></div>
   <h3>Extensible Runtimes</h3>
   <p>Use built-in TrainingRuntimes or build your own. Plugin architecture lets platform teams customize scheduling, networking, and resource management.</p>
   </div>

   <div class="feature-card">
   <div class="feature-icon"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="var(--lp-blue)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg></div>
   <h3>Production Ready</h3>
   <p>Battle-tested at scale by AWS, Red Hat, Oracle, and the broader CNCF ecosystem. Backed by the Kubeflow community with enterprise support.</p>
   </div>

   </div>
   </section>

   <section class="frameworks">
   <h2 class="section-title">Supported Frameworks</h2>
   <div class="framework-grid">
   <div class="framework-chip">PyTorch</div>
   <div class="framework-chip">JAX</div>
   <div class="framework-chip">DeepSpeed</div>
   <div class="framework-chip">MLX</div>
   <div class="framework-chip">HuggingFace</div>
   <div class="framework-chip">Megatron</div>
   <div class="framework-chip">XGBoost</div>
   <div class="framework-chip">TorchTune</div>
   </div>
   </section>

   <section class="quickstart">
   <h2 class="section-title">Train in 5 Lines</h2>
   <div class="code-block-wrapper">
   <div class="code-lang">python</div>
   <pre class="landing-code"><code><span class="hl-kw">from</span> kubeflow.trainer <span class="hl-kw">import</span> TrainerClient, CustomTrainer&#10;&#10;<span class="hl-fn">client</span> = TrainerClient()&#10;<span class="hl-fn">trainer</span> = CustomTrainer(func=<span class="hl-fn">my_train_func</span>, num_nodes=<span class="hl-num">4</span>)&#10;&#10;client.train(trainer=trainer)</code></pre>
   </div>
   <p class="quickstart-note">Same code runs locally with Docker or on any Kubernetes cluster. <a href="getting-started/index.html">See the full quickstart &rarr;</a></p>
   </section>

   <section class="community">
   <h2 class="section-title">Join the Community</h2>
   <p class="section-desc">We are an open and welcoming community of developers, data scientists, and organizations &mdash; backed by the Cloud Native Computing Foundation.</p>
   <div class="community-links">
   <a href="https://github.com/kubeflow/trainer" class="community-card"><span class="comm-icon comm-github"></span><strong>GitHub</strong><span>Star, fork, and contribute</span></a>
   <a href="https://kubeflow.slack.com" class="community-card"><span class="comm-icon comm-slack"></span><strong>Slack</strong><span>#kubeflow-trainer channel</span></a>
   <a href="https://bit.ly/2PWVCkV" class="community-card"><span class="comm-icon comm-cal"></span><strong>Meetings</strong><span>Bi-weekly community calls</span></a>
   <a href="https://blog.kubeflow.org/trainer/intro/" class="community-card"><span class="comm-icon comm-blog"></span><strong>Blog</strong><span>Latest news and tutorials</span></a>
   </div>
   </section>

   <footer class="landing-footer">
   <div class="footer-inner">
   <p>We are a <a href="https://www.cncf.io/">Cloud Native Computing Foundation</a> project.</p>
   <p class="footer-copy">&copy; 2026 The Kubeflow Authors &middot; Documentation distributed under CC BY 4.0</p>
   </div>
   </footer>

   </div>

.. toctree::
   :hidden:
   :maxdepth: 3

   Home <home>
   overview/index
   getting-started/index
   user-guides/index
   operator-guides/index
   contributor-guides/index
   legacy-v1/index
