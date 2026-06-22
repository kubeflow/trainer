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
   <img class="hero-logo" src="_images/trainer-logo.svg" alt="Kubeflow Trainer" />
   <p class="hero-tagline">The Kubernetes-native platform for distributed AI training and LLM fine-tuning at any scale.</p>
   <div class="hero-actions">
   <a href="getting-started/index.html" class="btn btn-primary">Get Started <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M5 12h14M12 5l7 7-7 7"/></svg></a>
   <a href="https://github.com/kubeflow/trainer" class="btn btn-secondary">GitHub</a>
   </div>
   <div class="hero-sub">Deploy anywhere you run Kubernetes &mdash; or train locally with Docker.</div>
   </div>
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
   <p>Used in production by organizations across the CNCF ecosystem. See <a href="https://github.com/kubeflow/trainer/blob/master/ADOPTERS.md">ADOPTERS.md</a> for adopters. Backed by the Kubeflow community with enterprise support.</p>
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

   <section class="doc-nav">
   <h2 class="section-title">Documentation</h2>
   <p class="section-desc">Browse guides by role — from your first TrainJob to production deployment and contribution.</p>
   <div class="doc-cards">
   <a class="doc-card" href="overview/index.html">
   <strong>Overview</strong>
   <p>Learn about Kubeflow Trainer, who it's for, and why you should use it</p>
   </a>
   <a class="doc-card" href="getting-started/index.html">
   <strong>Getting Started</strong>
   <p>Installation, first TrainJob, and quickstart tutorials</p>
   </a>
   <a class="doc-card" href="user-guides/index.html">
   <strong>User Guides</strong>
   <p>Documentation for AI practitioners and ML engineers using Kubeflow Trainer</p>
   </a>
   <a class="doc-card" href="operator-guides/index.html">
   <strong>Operator Guides</strong>
   <p>Documentation for platform administrators deploying and managing Kubeflow Trainer</p>
   </a>
   <a class="doc-card" href="contributor-guides/index.html">
   <strong>Contributor Guides</strong>
   <p>Architecture, development workflow, and how to extend Kubeflow Trainer</p>
   </a>
   <a class="doc-card" href="legacy-v1/index.html">
   <strong>Legacy Kubeflow Training Operator (v1)</strong>
   <p>Kubeflow Training Operator v1 documentation — archived guides, installation, and migration to v2</p>
   </a>
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
   <a href="https://app.slack.com/client/T08PSQ7BQ/C0742LDFZ4K" class="community-card"><span class="comm-icon comm-slack"></span><strong>Slack</strong><span>#kubeflow-trainer on CNCF Slack</span></a>
   <a href="https://slack.cncf.io/" class="community-card"><span class="comm-icon comm-cncf"></span><strong>CNCF Slack</strong><span>Join the CNCF Slack workspace</span></a>
   <a href="https://groups.google.com/g/kubeflow-discuss" class="community-card"><span class="comm-icon comm-mail"></span><strong>Mailing List</strong><span>kubeflow-discuss</span></a>
   <a href="https://bit.ly/2PWVCkV" class="community-card"><span class="comm-icon comm-cal"></span><strong>Meeting Notes</strong><span>Trainer &amp; Katib community calls</span></a>
   <a href="https://www.youtube.com/playlist?list=PLmzRWLV1CK_xAiAY-3Vw94lrUs4xeNZ3j" class="community-card"><span class="comm-icon comm-youtube"></span><strong>Recordings</strong><span>Watch past community meetings</span></a>
   <a href="https://zoom-lfx.platform.linuxfoundation.org/meetings/kubeflow" class="community-card"><span class="comm-icon comm-calendar"></span><strong>Community Calendar</strong><span>View all Kubeflow meetings</span></a>
   <a href="https://webcal.prod.itx.linuxfoundation.org/lfx/a092M00001LkNgVQAV" class="community-card"><span class="comm-icon comm-ical"></span><strong>Add to Calendar</strong><span>Subscribe via iCal</span></a>
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

.. only:: html

   .. Ensure Sphinx copies the logo to _images/
   .. image:: images/trainer-logo.svg
      :width: 0
      :class: hidden

.. toctree::
   :hidden:
   :maxdepth: 3

   overview/index
   getting-started/index
   user-guides/index
   operator-guides/index
   contributor-guides/index
   legacy-v1/index
