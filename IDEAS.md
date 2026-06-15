# InsightPilot — Feature Ideas

A collection of features, improvements, and capabilities to evolve InsightPilot into a full-fledged AI-native BI platform.

---

## 🚀 High Impact / Core Differentiators

### 1. Conversational Multi-Turn Analysis
Currently each query is standalone. Enable a threaded conversation where users can ask follow-ups:
- *"Filter by region APAC"*
- *"Drill down into Q4 data"*
- *"Show me just the top 5 products"*
- *"Compare this to last month"*

The analyzer should maintain context across turns — remembering previous filters, groupings, and results — so analysis feels like a real conversation, not isolated requests.

### 2. AI Narrative & Insights
After computing KPIs and charts, have the LLM generate a written summary:
> "Revenue grew 23% QoQ, driven primarily by the APAC region which saw a 41% increase. Customer churn decreased by 5%, though support ticket volume rose 12% in EMEA."

This turns raw numbers into a story users can share and act on.

### 3. Data Transformation Pipeline
In-app ETL capabilities so users don't need to clean data externally:
- Filter rows (by condition)
- Rename / drop columns
- Handle null values (fill, drop, interpolate)
- Derive new columns (e.g. `profit = revenue - cost`)
- Aggregate / pivot data
- Join two datasets on a key column
- Undo/redo for transformation steps

### 4. Cross-Dataset Joins & Relationships
Users upload two CSVs (e.g. `orders.csv` + `customers.csv`) and ask questions like *"show me revenue by customer region"*. Let them define join keys, then run analysis across merged data.

### 5. Custom SQL Query Mode
Power users want to write arbitrary SQL against uploaded datasets or connected databases. Provide:
- A SQL editor with syntax highlighting
- Table schema explorer
- Result preview (paginated)
- One-click "visualize this" to chart the query result

### 6. Scheduled Reports & Alerts
Turn BI from reactive to proactive:
- Schedule dashboard snapshots via email (daily/weekly/monthly)
- Alert rules: *"email me when revenue drops more than 10%"*
- Slack/Teams webhook integration
- PDF snapshot of the dashboard attached to the report

### 7. Drag-and-Drop Dashboard Editor
Let users customize layout freely:
- Resize and position charts on a grid
- Add text blocks, dividers, images
- Choose chart type per tile (swap between bar/line/pie/table)
- Save layout as a template

---

## 📈 Medium Impact / Polishes

### 8. More Interactive Chart Types
Python already generates line, area, scatter, heatmap, box plots. Bring these into the frontend with Recharts for interactivity:
- **Line chart** — time series
- **Area chart** — cumulative trends
- **Scatter plot** — correlation exploration
- **Heatmap** — density / cross-tabs
- **Combo chart** — bar + line overlay

### 9. Anomaly & Outlier Detection
Auto-flag unusual data points using statistical methods (z-score, IQR, moving average deviation):
- Highlight anomalies on charts
- Show a dedicated "Anomalies" section in analysis results
- LLM explains what might have caused them

### 10. Smart Auto-Visualization
Instead of hardcoded KPI/trend/segment charts, let the LLM decide the best visualization for each query:
- *"Show me revenue by month"* → Line chart
- *"Compare sales by region"* → Bar chart
- *"Show distribution of order values"* → Histogram
- *"Show relationship between ad spend and revenue"* → Scatter

### 11. Explainable AI (Query Transparency)
For every KPI or chart, show the logic behind it:
- Generated SQL or pandas code
- Column selections and groupings used
- Warning flags (e.g. "only 3 data points — trend may be unreliable")

Builds user trust in the AI.

### 12. Data Profiler
Before analysis, allow users to explore their dataset:
- Column statistics (min, max, mean, null count, unique values)
- Distribution histograms
- Correlation matrix
- Duplicate detection
- Data type summary

### 13. Theme & Dark Mode
Visual polish that users notice:
- Dark mode toggle (persisted in localStorage)
- Accessible color palettes
- Custom accent colors
- Chart color scheme selection

### 14. Multiple LLM Provider Support
Let users choose their AI engine:
- OpenAI (GPT-4, GPT-4o)
- Anthropic (Claude 3.5 Sonnet)
- Google (Gemini Pro)
- Local / Ollama models
- Configurable per-query or per-user

---

## 🔧 Infrastructure & Scale

### 15. Full Persistent Storage
Datasets, connections, and share tokens are currently in-memory — lost on restart. Move them to Supabase/PostgreSQL:
- Dataset metadata + row storage
- Connection configs (already encrypted)
- Share tokens
- User preferences and theme settings

### 16. Full Database Connectors
Only PostgreSQL works today. Implement the remaining connectors:
- **MySQL** — native Go driver (`go-sql-driver/mysql`)
- **BigQuery** — REST API via Google SDK
- **Snowflake** — native Go driver
- **Redshift** — PostgreSQL-compatible (mostly works but needs testing)
- **SQLite** — file upload alternative

### 17. Role-Based Access Control (RBAC)
Multi-user teams need clear permissions:
- **Admin** — manage users, all data
- **Editor** — create/edit dashboards, upload data
- **Viewer** — view only, cannot modify
- Dataset-level sharing between users

### 18. Embedded Dashboards (Public Sharing)
Allow dashboards to be shared publicly via token or iframe:
- Time-limited share links (already partially implemented)
- Embed snippet: `<iframe src="https://app.insightpilot.dev/share/abc123">`
- No-auth viewer mode

Useful for customer-facing analytics.

### 19. Docker Compose Setup
One-command deployment:
- `docker-compose up` starts Go backend + frontend + Postgres + Python runtime
- Environment variable templating
- Health checks and orchestration

### 20. API Documentation (OpenAPI / Swagger)
Expose an auto-generated OpenAPI spec so developers can build on top of InsightPilot:
- Swagger UI at `/api/docs`
- TypeScript client generation
- API key authentication for programmatic access

---

## ✨ Nice-to-Have / Future

### 21. Excel & PowerPoint Export
Beyond CSV and PDF:
- **Excel** — multi-sheet export with raw data + charts as images
- **PowerPoint** — each chart on its own slide with title and commentary

### 22. Collaborative Annotations
Let team members comment on charts and dashboards:
- Pin comments to specific data points
- @mention team members
- Resolve threads

### 23. Natural Language Data Cleaning
Describe cleaning operations in plain English:
- *"Remove rows where price is 0"*
- *"Fill missing values in 'region' with 'Unknown'"*
- *"Split 'full_name' into first and last name"*
LLM translates these into transformation steps.

### 24. Dashboard Version History
Track changes to dashboards:
- Snapshot on every save
- Rollback to any previous version
- Diff view showing what changed

### 25. What-If Analysis
Let users adjust variables and see projected impact:
- *"What if we increase prices by 10%?"*
- *"What if marketing spend doubles?"*
LLM or a simple projection model shows the result.

### 26. Data Source Change Detection
Monitor connected databases for changes and auto-refresh dashboards:
- Poll for new rows every N minutes
- Webhook endpoint for push notifications
- Badge indicating "stale" data

### 27. Mobile-Responsive Dashboards
Optimize the dashboard view for mobile devices:
- Stacked single-column layout
- Touch-friendly interactions
- Simplified charts

### 28. Multi-Language Internationalization (i18n)
Support for non-English users:
- UI translations (i18next)
- LLM prompts in the user's language
- Number/date formatting per locale

### 29. Usage Analytics
Track feature usage to guide development:
- Most common queries and chart types
- Upload volume and dataset size distribution
- Error rates and latency metrics

### 30. AI-Powered Dashboard Builder
Describe a dashboard in natural language and have it built automatically:
- *"Build me a sales dashboard with revenue trend, top products, and regional breakdown"*
- AI creates charts, arranges layout, and selects KPIs

---

> **How to prioritize**: Start with features that (a) solve a real user pain, (b) differentiate from existing BI tools, and (c) are achievable within your current architecture. The transformation pipeline, cross-dataset joins, and conversational multi-turn analysis offer the highest ROI for the next development cycle.
