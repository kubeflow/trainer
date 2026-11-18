# Copyright The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Inject the Kubeflow Trainer top navigation bar via a Sphinx extension."""

from pathlib import Path

TOPNAV_HTML = (Path(__file__).parent / "topnav.html").read_text(encoding="utf-8")


def render_topnav(app, pagename, templatename, context, doctree):
    # Resolve internal links relative to the current page, so they work both locally
    # and on ReadTheDocs, where pages are served under a version prefix (e.g. /en/latest/).
    pathto = context["pathto"]
    html = TOPNAV_HTML.replace("__TOPNAV_ROOT_URL__", pathto("index"))
    html = html.replace("__TOPNAV_OVERVIEW_URL__", pathto("overview/index"))
    html = html.replace("__TOPNAV_COMMUNITY_URL__", pathto("community/index"))
    context["theme_announcement"] = html


def setup(app):
    # Enable Furo's announcement slot; the per-page HTML is rendered in render_topnav.
    app.config.html_theme_options["announcement"] = TOPNAV_HTML
    app.connect("html-page-context", render_topnav)
    return {
        "version": "0.1",
        "parallel_read_safe": True,
        "parallel_write_safe": True,
    }
