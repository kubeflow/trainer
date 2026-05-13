# Configuration file for the Sphinx documentation builder.
# Kubeflow Trainer Documentation System

import os

# -- Project information -----------------------------------------------------
project = "Kubeflow Trainer"
copyright = "2026, Kubeflow Authors"
author = "Kubeflow Authors"

# The version is set from environment variable or defaults to "latest"
# ReadTheDocs sets READTHEDOCS_VERSION automatically
version = os.getenv("READTHEDOCS_VERSION", "latest")
release = version

# -- General configuration ---------------------------------------------------
extensions = [
    "myst_parser",  # Markdown support via MyST
    "sphinxcontrib.mermaid",  # Mermaid diagram rendering
    # "autoapi.extension",  # Auto-generate Python API reference (disabled until SDK integration)
    "sphinx_copybutton",  # Copy button on code blocks
    "sphinx_design",  # Grid layouts and card components
]

# Add any paths that contain templates here, relative to this directory.
templates_path = ["_templates"]

# List of patterns, relative to source directory, that match files and
# directories to ignore when looking for source files.
exclude_patterns = [
    "_build",
    "Thumbs.db",
    ".DS_Store",
    "*.egg-info",
    "__pycache__",
    "proposals",  # Exclude pre-existing proposals directory
    "images",  # Exclude pre-existing images directory
    "release",  # Exclude pre-existing release directory
    "README.md",  # Exclude pre-existing README
]

# -- Options for HTML output -------------------------------------------------
html_theme = "furo"
html_title = "Kubeflow Trainer"
html_static_path = ["_static"]
html_css_files = ["css/custom.css"]
html_js_files = ["js/external-links.js", "js/sidebar-toggle.js"]

# Furo theme options
html_theme_options = {
    "light_css_variables": {
        "color-brand-primary": "#4299e1",
        "color-brand-content": "#3182ce",
    },
    "dark_css_variables": {
        "color-brand-primary": "#63b3ed",
        "color-brand-content": "#63b3ed",
    },
    "sidebar_hide_name": False,
    "navigation_with_keys": True,
    "top_of_page_buttons": ["view", "edit"],
    "source_repository": "https://github.com/kubeflow/trainer",
    "source_branch": "master",
    "source_directory": "docs/",
    "announcement": (
        "<nav class='top-nav'>"
        "<a href='/index.html' class='top-nav-brand'>"
        "<svg class='top-nav-logo' width='28' height='28'"
        " viewBox='0 0 256 256'"
        " xmlns='http://www.w3.org/2000/svg'>"
        "<g transform='matrix(1.2742 0 0 1.2745 -46.441 11.393)'>"
        "<path d='m95.9 62.15 4.1 102.1 73.75-94.12"
        "a6.79 6.79 0 0 1 9.6-1.11l46 36.92-15-65.61z'"
        " fill='#4279f4'/>"
        "<path d='m102.55 182.98h65.42l-40.17-32.23z'"
        " fill='#0028aa'/>"
        "<path d='m180.18 83.92-44 56.14 46.88 37.61"
        " 44.47-55.76z' fill='#014bd1'/>"
        "<path d='m83.56 52.3 0.01-0.01 38.69-48.52"
        "-62.39 30.05-15.41 67.51z' fill='#bedcff'/>"
        "<path d='m45.32 122.05 41.44 51.96-3.95-98.98z'"
        " fill='#6ca1ff'/>"
        "<path d='m202.31 28.73-59.66-28.73-37.13 46.56z'"
        " fill='#a1c3ff'/>"
        "</g></svg>"
        "<span>Kubeflow Trainer</span>"
        "</a>"
        "<div class='top-nav-links'>"
        "<a href='https://github.com/kubeflow/trainer/blob/"
        "master/examples/pytorch/image-classification/"
        "mnist.ipynb'>Examples</a>"
        "<a href='https://github.com/kubeflow/trainer'>"
        "GitHub</a>"
        "<a href='https://kubeflow.slack.com'>Slack</a>"
        "<a href='https://blog.kubeflow.org/trainer/intro/'>"
        "Blog</a>"
        "</div></nav>"
    ),
}

# ReadTheDocs version switcher integration
# These variables are set by ReadTheDocs at build time
html_context = {
    "display_github": True,
    "github_user": "kubeflow",
    "github_repo": "trainer",
    "github_version": "master",
    "conf_py_path": "/docs/",
}

# -- MyST Parser configuration -----------------------------------------------
myst_enable_extensions = [
    "colon_fence",  # ::: fence syntax for directives
    "deflist",  # Definition lists
    "fieldlist",  # Field lists
    "substitution",  # Variable substitution
    "tasklist",  # Task lists [ ] [x]
]
myst_links_external_new_tab = True
myst_heading_anchors = 4

# -- Mermaid configuration ---------------------------------------------------
mermaid_version = "11.5.0"  # Use specific version for reproducibility
mermaid_init_js = """
mermaid.initialize({
    theme: 'base',
    themeVariables: {
        primaryColor: '#326CE5',
        primaryTextColor: '#fff',
        primaryBorderColor: '#1a4b99',
        lineColor: '#326CE5',
        secondaryColor: '#f0f0f0',
        tertiaryColor: '#fff'
    }
});
"""

# -- Link checking configuration ---------------------------------------------
linkcheck_ignore = [
    r"http://localhost:\d+/",  # Ignore localhost links
    r"https://github\.com/.*/pulls/.*",  # GitHub PR links may be private
    r"https://medium\.com/.*",  # Medium blocks automated link checking (403)
]
linkcheck_anchors_ignore = [
    r"utilizing-nn-average-gradients",  # MLX docs anchor not stable
]

# -- Copy button configuration -----------------------------------------------
copybutton_prompt_text = r">>> |\.\.\. |\$ |In \[\d*\]: | {2,5}\.\.\.: | {5,8}: "
copybutton_prompt_is_regexp = True
copybutton_line_continuation_character = "\\"
