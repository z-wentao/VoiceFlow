package templates

import (
    "fmt"
    "html/template"
    "strings"
    "time"

    "github.com/z-wentao/voiceflow/pkg/models"
)

// FormatTime 格式化时间
func FormatTime(t time.Time) string {
    now := time.Now()
    diff := now.Sub(t)

    if diff < time.Minute {
	return "刚刚"
    }
    if diff < time.Hour {
	return fmt.Sprintf("%d 分钟前", int(diff.Minutes()))
    }
    if diff < 24*time.Hour {
	return fmt.Sprintf("%d 小时前", int(diff.Hours()))
    }
    return t.Format("2006-01-02 15:04")
}

// IsVideoFile 判断是否是视频文件
func IsVideoFile(filename string) bool {
    ext := strings.ToLower(filename[strings.LastIndex(filename, "."):])
    videoExts := []string{".mp4", ".webm", ".ogg", ".mov", ".avi", ".mkv", ".wmv", ".flv", ".m4v"}
    for _, ve := range videoExts {
	if ext == ve {
	    return true
	}
    }
    return false
}

// GetMediaIcon 获取媒体图标
func GetMediaIcon(filename string) string {
    if IsVideoFile(filename) {
	return "🎬"
    }
    return "🎵"
}

// RenderTaskCard 渲染任务卡片
func RenderTaskCard(job *models.TranscriptionJob) template.HTML {
    statusText := map[string]string{
	"pending":    "等待处理",
	"processing": "处理中",
	"completed":  "已完成",
	"failed":     "失败",
    }
    status := statusText[string(job.Status)]
    if status == "" {
	status = "未知"
    }

    spinner := ""
    if job.Status == "processing" {
	spinner = "<span>⏳</span>"
    }

    progress := ""
    if job.Progress > 0 {
	progress = fmt.Sprintf("<span>进度: %d%%</span>", job.Progress)
    }

    actions := fmt.Sprintf(`
	<button onclick="togglePlayer('%s')">%s 播放</button>
	`, job.JobID, GetMediaIcon(job.Filename))

    if job.Status == "completed" {
	actions += fmt.Sprintf(`
	    <button hx-get="/api/jobs/%s/download">📥 下载文本</button>
	    `, job.JobID)

	// 如果有字幕文件，显示下载字幕按钮
	if job.SubtitlePath != "" {
	    actions += fmt.Sprintf(`
		<button hx-get="/api/jobs/%s/download-subtitle">🎬 下载字幕</button>
		`, job.JobID)
	}

	actions += fmt.Sprintf(`
	    <button hx-post="/api/jobs/%s/extract-vocabulary"
	    hx-target="#details-%s"
	    hx-swap="innerHTML">📚 提取单词</button>
	    `, job.JobID, job.JobID)
    }

    actions += fmt.Sprintf(`
	<button hx-delete="/api/jobs/%s"
	hx-confirm="确定删除？"
	hx-target="#task-%s"
	hx-swap="outerHTML">🗑️ 删除</button>
	<button hx-get="/api/jobs/%s/details"
	hx-target="#details-%s"
	hx-swap="innerHTML">▼ 详情</button>
	`, job.JobID, job.JobID, job.JobID, job.JobID)

    html := fmt.Sprintf(`
	<div class="task-card" data-job-id="%s" data-status="%s" id="task-%s">
	<hr>
	<p><strong>%s</strong> %s</p>
	<p>状态: <strong>%s</strong> | %s | 时间: %s</p>
	<p>%s</p>
	<div id="details-%s"></div>
	</div>
	`,
	job.JobID,
	job.Status,
	job.JobID,
	template.HTMLEscapeString(job.Filename),
	spinner,
	status,
	progress,
	FormatTime(job.CreatedAt),
	actions,
	job.JobID,
	)

    return template.HTML(html)
}

// RenderTaskDetails 渲染任务详情
func RenderTaskDetails(job *models.TranscriptionJob) template.HTML {
    var html strings.Builder

    html.WriteString("<hr>")

    // 媒体播放器
    html.WriteString(fmt.Sprintf(`
	<div id="player-%s" hidden>
	<h4>%s</h4>
	%s
	</div>
	`, job.JobID, GetMediaIcon(job.Filename), renderMediaPlayer(job)))

    // 进度条
    if (job.Status == "processing" || job.Status == "completed") && job.Progress > 0 {
	html.WriteString(fmt.Sprintf(`
	    <div>
	    <p>转换进度: %d%%</p>
	    <progress value="%d" max="100"></progress>
	    </div>
	    `, job.Progress, job.Progress))
    }

    // 转录结果
    if job.Status == "completed" && job.Result != "" {
	html.WriteString(fmt.Sprintf(`
	    <div>
	    <h4>转录结果</h4>
	    <textarea rows="15" cols="100" readonly>%s</textarea>
	    </div>
	    `, template.HTMLEscapeString(job.Result)))
    }

    // 错误信息
    if job.Status == "failed" && job.Error != "" {
	html.WriteString(fmt.Sprintf(`
	    <div>
	    <p><strong>错误:</strong> %s</p>
	    </div>
	    `, template.HTMLEscapeString(job.Error)))
    }

    // 单词列表
    if job.Status == "completed" && len(job.VocabDetail) > 0 {
	html.WriteString(renderVocabulary(job))
    }

    return template.HTML(html.String())
}

// renderMediaPlayer 渲染媒体播放器（支持字幕）
func renderMediaPlayer(job *models.TranscriptionJob) string {
    if IsVideoFile(job.Filename) {
	// 视频播放器容器（使用自定义字幕渲染）
	player := fmt.Sprintf(`
	    <style>
	    #video-container-%s:fullscreen {
	    width: 100vw;
	    height: 100vh;
	    background: black;
	    display: flex;
	    align-items: center;
	    justify-content: center;
	    }
	    #video-container-%s:fullscreen video {
	    width: 100%%;
	    height: 100%%;
	    }
	    #video-container-%s:fullscreen #subtitle-%s {
	    font-size: 24px;
	    }
	    </style>
	    <div id="video-container-%s" style="position: relative; display: inline-block; max-width: 100%%;">
	    <video id="video-%s" controls crossorigin="anonymous" src="/%s" style="max-width: 100%%; display: block;"></video>`,
	    job.JobID, job.JobID, job.JobID, job.JobID, job.JobID, job.JobID, job.FilePath)

	if job.VTTPath != "" && job.Status == models.StatusCompleted {
	    // 添加字幕容器（DOM 元素，插件可以访问）
	    player += fmt.Sprintf(`
		<!-- 隐藏的字幕列表，供翻译插件预读取和翻译 -->
		<div id="subtitle-list-%s" style="display: none;" lang="en"></div>
		<!-- 显示的字幕容器 -->
		<div id="subtitle-%s" style="position: absolute; bottom: 60px; left: 0; right: 0; text-align: center; pointer-events: none;"></div>
		</div>
		<script>
		(function() {
		const video = document.getElementById('video-%s');
		const subtitleDiv = document.getElementById('subtitle-%s');
		const subtitleList = document.getElementById('subtitle-list-%s');
		let subtitles = [];
		let currentCueIndex = -1;

		// 加载并解析 VTT 字幕文件
		fetch('/api/jobs/%s/subtitle.vtt')
		.then(response => response.text())
		.then(vttContent => {
		// 解析 VTT 格式
		subtitles = parseVTT(vttContent);
		console.log('字幕已加载:', subtitles.length, '条');

		// 创建隐藏的字幕列表（供翻译插件预读取）
		renderHiddenSubtitleList();
		})
		.catch(err => console.error('加载字幕失败:', err));

		// 简单的 VTT 解析器
		function parseVTT(vtt) {
		const lines = vtt.split('\n');
		const cues = [];
		let i = 0;

		while (i < lines.length) {
		const line = lines[i].trim();

		// 跳过 WEBVTT 头和空行
		if (line === 'WEBVTT' || line === '' || /^\d+$/.test(line)) {
		i++;
		continue;
		}

		// 时间戳行格式: 00:00:00.000 --> 00:00:05.000
		if (line.includes('-->')) {
		const [startStr, endStr] = line.split('-->').map(s => s.trim());
		const start = parseTime(startStr);
		const end = parseTime(endStr);

		// 下一行是字幕文本
		i++;
		let text = '';
		while (i < lines.length && lines[i].trim() !== '') {
		text += lines[i].trim() + ' ';
		i++;
		}

		cues.push({ start, end, text: text.trim() });
		}
		i++;
		}
		return cues;
		}

		// 解析时间字符串 (HH:MM:SS.mmm) 为秒
		function parseTime(timeStr) {
		const parts = timeStr.split(':');
		const hours = parseInt(parts[0]);
		const minutes = parseInt(parts[1]);
		const seconds = parseFloat(parts[2]);
		return hours * 3600 + minutes * 60 + seconds;
		}

		// 渲染隐藏的字幕列表（供翻译插件预读取）
		function renderHiddenSubtitleList() {
		subtitles.forEach((cue, index) => {
		const p = document.createElement('p');
		p.setAttribute('lang', 'en');
		p.setAttribute('translate', 'yes');
		p.setAttribute('data-subtitle-index', index);
		p.textContent = cue.text;
		subtitleList.appendChild(p);
		});
		console.log('隐藏字幕列表已创建，翻译插件可以预读取', subtitles.length, '条字幕');
		}

		// 处理全屏：让整个容器全屏，而不是只有视频
		const videoContainer = document.getElementById('video-container-%s');

		// 双击视频进入/退出全屏
		video.addEventListener('dblclick', function(e) {
		e.preventDefault();
		if (!document.fullscreenElement) {
		videoContainer.requestFullscreen().catch(err => {
		console.error('全屏失败:', err);
		});
		} else {
		document.exitFullscreen();
		}
		});

		// 视频播放时更新字幕
		video.addEventListener('timeupdate', function() {
		const currentTime = video.currentTime;
		let foundCueIndex = -1;

		// 查找当前时间对应的字幕
		for (let i = 0; i < subtitles.length; i++) {
		if (currentTime >= subtitles[i].start && currentTime <= subtitles[i].end) {
		foundCueIndex = i;
		break;
		}
		}

		// 只在字幕切换时更新 DOM（删除旧元素，创建新元素）
		if (foundCueIndex !== currentCueIndex) {
		currentCueIndex = foundCueIndex;

		// 清空容器
		subtitleDiv.innerHTML = '';

		// 如果有字幕，从隐藏列表中克隆对应的元素
		if (foundCueIndex >= 0) {
		const hiddenSubtitle = subtitleList.querySelector('[data-subtitle-index="' + foundCueIndex + '"]');

		if (hiddenSubtitle) {
		// 克隆隐藏的字幕元素（包含翻译插件添加的翻译内容）
		const span = document.createElement('span');
		span.style.cssText = 'background: rgba(0,0,0,0.8); color: white; padding: 5px 10px; border-radius: 3px; font-size: 18px; display: inline-block; max-width: 90%%; word-wrap: break-word;';
		span.setAttribute('lang', 'en');
		span.setAttribute('translate', 'yes');
		span.setAttribute('data-subtitle-index', foundCueIndex);

		// 复制隐藏元素的内容（可能包含翻译）
		span.innerHTML = hiddenSubtitle.innerHTML || hiddenSubtitle.textContent;

		// 插入显示区域
		subtitleDiv.appendChild(span);
		}
		}
		}
		});
		})();
		</script>`, job.JobID, job.JobID, job.JobID, job.JobID, job.JobID, job.JobID, job.JobID)
	} else {
	    player += `</div>`
	}
	return player
    }

    // 音频播放器（暂不支持字幕显示，但可以下载）
    return fmt.Sprintf(`<audio controls src="/%s"></audio>`, job.FilePath)
}

// renderVocabulary 渲染单词列表
func renderVocabulary(job *models.TranscriptionJob) string {
    var html strings.Builder

    html.WriteString(fmt.Sprintf(`
	<div>
	<hr>
	<h4>📚 提取的单词 (%d)</h4>
	<button onclick="showMaimemoForm('%s')">🔄 同步到墨墨</button>
	<ul>
	`, len(job.VocabDetail), job.JobID))

    for _, word := range job.VocabDetail {
	example := ""
	if word.Example != "" {
	    example = fmt.Sprintf("<br><em>%s</em>", template.HTMLEscapeString(word.Example))
	}
	html.WriteString(fmt.Sprintf(`
	    <li>
	    <strong>%s</strong><br>
	    %s%s
	    </li>
	    `, template.HTMLEscapeString(word.Word), template.HTMLEscapeString(word.Definition), example))
    }

    html.WriteString("</ul>")
    html.WriteString(renderMaimemoForm(job.JobID))
    html.WriteString("</div>")

    return html.String()
}

// renderMaimemoForm 渲染墨墨同步表单
func renderMaimemoForm(jobID string) string {
    return fmt.Sprintf(`
	<div id="maimemo-form-%s" hidden>
	<hr>
	<h4>同步到墨墨背单词</h4>
	<input type="hidden" id="job-id-%s" name="job_id" value="%s">
	<label>墨墨 API Token:</label>
	<input type="text" id="token-%s" name="token" placeholder="输入 Token" onchange="saveToken(this.value)">
	<br>
	<label>云词本 ID:</label>
	<input type="text" id="notepad-%s" name="notepad_id" placeholder="输入云词本 ID" onchange="saveNotepadId(this.value)">
	<button hx-post="/api/maimemo/list-notepads"
	hx-include="#token-%s, #job-id-%s"
	hx-target="#notepad-list-%s"
	hx-swap="innerHTML"
	onclick="document.getElementById('notepad-list-%s').hidden = false">🔍 查询云词本</button>
	<div id="notepad-list-%s" hidden style="margin-top: 10px; padding: 10px; border: 1px solid #ddd; border-radius: 4px; max-height: 200px; overflow-y: auto;"></div>
	<br>
	<button hx-post="/api/jobs/%s/sync-to-maimemo"
	hx-include="#token-%s, #notepad-%s"
	hx-target="#sync-result-%s"
	hx-swap="innerHTML"
	hx-confirm="确定同步？">确认同步</button>
	<button onclick="hideMaimemoForm('%s')">取消</button>
	<div id="sync-result-%s" style="margin-top: 10px;"></div>
	</div>
	`, jobID, jobID, jobID, jobID, jobID, jobID, jobID, jobID, jobID, jobID, jobID, jobID, jobID, jobID, jobID, jobID)
}

// RenderNotepads 渲染云词本列表
func RenderNotepads(notepads []map[string]interface{}, jobID string) template.HTML {
    if len(notepads) == 0 {
	return template.HTML("<p style='color: #666; padding: 10px;'>没有云词本</p>")
    }

    var html strings.Builder
    html.WriteString("<p style='margin: 0 0 8px 0; font-size: 12px; color: #666;'>点击选择云词本：</p>")
    html.WriteString("<ul style='list-style: none; margin: 0; padding: 0;'>")
    for _, notepad := range notepads {
	id := notepad["id"].(string)
	title := notepad["title"].(string)
	html.WriteString(fmt.Sprintf(`
	    <li onclick="selectNotepad('%s', '%s')" style="padding: 8px 12px; margin: 4px 0; background: #f5f5f5; border-radius: 4px; cursor: pointer; transition: background 0.2s;" onmouseover="this.style.background='#e8e8e8'" onmouseout="this.style.background='#f5f5f5'">
		<strong>%s</strong><br>
		<small style="color: #666;">ID: %s</small>
	    </li>
	    `, jobID, id, template.HTMLEscapeString(title), id))
    }
    html.WriteString("</ul>")

    return template.HTML(html.String())
}

// RenderTasksList 渲染任务列表
func RenderTasksList(jobs []*models.TranscriptionJob) template.HTML {
    if len(jobs) == 0 {
	return template.HTML("<p>暂无任务</p>")
    }

    var html strings.Builder
    for _, job := range jobs {
	html.WriteString(string(RenderTaskCard(job)))
    }

    return template.HTML(html.String())
}
