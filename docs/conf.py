# Configuration file for the Sphinx documentation builder.
# Kubeflow Trainer Documentation System

import os
import sys

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
    "images",     # Exclude pre-existing images directory
    "release",    # Exclude pre-existing release directory
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
        "color-brand-primary": "#326CE5",  # Kubernetes blue
        "color-brand-content": "#326CE5",
    },
    "dark_css_variables": {
        "color-brand-primary": "#5B9DF1",  # Lighter blue for dark mode
        "color-brand-content": "#5B9DF1",
    },
    "sidebar_hide_name": False,
    "navigation_with_keys": True,
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
