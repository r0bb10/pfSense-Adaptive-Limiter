<?php

require_once('guiconfig.inc');

const ADAPTIVE_LIMITER_STATUS_FILE = '/var/run/adaptive-limiter/status.json';

if (isset($_GET['ajax'])) {
	header('Content-Type: application/json');
	header('Cache-Control: no-store');
	if (!is_readable(ADAPTIVE_LIMITER_STATUS_FILE)) {
		http_response_code(503);
		echo json_encode(['error' => 'Status is unavailable. The service may be stopped.']);
		exit;
	}
	$data = file_get_contents(ADAPTIVE_LIMITER_STATUS_FILE);
	if (json_decode($data, true) === null) {
		http_response_code(503);
		echo json_encode(['error' => 'The daemon status file is invalid.']);
		exit;
	}
	echo $data;
	exit;
}

$pgtitle = [gettext('Status'), gettext('Adaptive Limiter')];
include('head.inc');
?>

<ul class="nav nav-tabs">
	<li><a href="/pkg_edit.php?xml=adaptive-limiter.xml"><?=gettext('Settings')?></a></li>
	<li class="active"><a href="/status_adaptive_limiter.php"><?=gettext('Status')?></a></li>
</ul>

<div class="panel panel-default">
	<div class="panel-heading"><h2 class="panel-title"><?=gettext('Runtime Status')?></h2></div>
	<div class="panel-body">
		<div id="adaptive-limiter-error" class="alert alert-warning" style="display:none"></div>
		<table class="table table-striped table-condensed">
			<tbody>
				<tr><th><?=gettext('Version')?></th><td id="al-version">-</td></tr>
				<tr><th><?=gettext('Mode')?></th><td id="al-mode">-</td></tr>
				<tr><th><?=gettext('WAN Interface')?></th><td id="al-interface">-</td></tr>
				<tr><th><?=gettext('Updated')?></th><td id="al-updated">-</td></tr>
				<tr><th><?=gettext('Download')?></th><td id="al-download">-</td></tr>
				<tr><th><?=gettext('Upload')?></th><td id="al-upload">-</td></tr>
				<tr><th><?=gettext('Latency')?></th><td id="al-latency">-</td></tr>
				<tr><th><?=gettext('Healthy Reflectors')?></th><td id="al-reflectors">-</td></tr>
				<tr><th><?=gettext('Last Decision')?></th><td id="al-reason">-</td></tr>
			</tbody>
		</table>
	</div>
</div>

<script>
function alText(id, value) {
	document.getElementById(id).textContent = value;
}

async function refreshAdaptiveLimiter() {
	const errorBox = document.getElementById('adaptive-limiter-error');
	try {
		const response = await fetch('/status_adaptive_limiter.php?ajax=1', {cache: 'no-store'});
		const data = await response.json();
		if (!response.ok || data.error) {
			throw new Error(data.error || 'Status request failed');
		}
		errorBox.style.display = 'none';
		alText('al-version', data.version);
		alText('al-mode', data.mode);
		alText('al-interface', data.wan_interface);
		alText('al-updated', data.updated_at);
		alText('al-download', `${data.download.current_mbps.toFixed(1)} Mb/s, pipe ${data.download.pipe}, ${data.download.state}`);
		alText('al-upload', `${data.upload.current_mbps.toFixed(1)} Mb/s, pipe ${data.upload.pipe}, ${data.upload.state}`);
		alText('al-latency', `${data.current_rtt_ms.toFixed(2)} ms (${data.delay_delta_ms.toFixed(2)} ms above baseline)`);
		alText('al-reflectors', data.healthy_reflectors);
		alText('al-reason', data.last_reason);
	} catch (error) {
		errorBox.textContent = error.message;
		errorBox.style.display = 'block';
	}
}

refreshAdaptiveLimiter();
setInterval(refreshAdaptiveLimiter, 2000);
</script>

<?php include('foot.inc'); ?>
