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
				<tr><th><?=gettext('Last Error')?></th><td id="al-error">-</td></tr>
			</tbody>
		</table>
	</div>
</div>

<div class="panel panel-default">
	<div class="panel-heading"><h2 class="panel-title"><?=gettext('Reflectors')?></h2></div>
	<div class="panel-body table-responsive">
		<table class="table table-striped table-condensed">
			<thead><tr><th><?=gettext('Address')?></th><th><?=gettext('Health')?></th><th><?=gettext('RTT')?></th><th><?=gettext('Baseline')?></th><th><?=gettext('Delta')?></th><th><?=gettext('Last Error')?></th></tr></thead>
			<tbody id="al-reflector-list"><tr><td colspan="6">-</td></tr></tbody>
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
		const body = await response.text();
		let data;
		try {
			data = JSON.parse(body);
		} catch (error) {
			throw new Error(`Status endpoint returned HTTP ${response.status} instead of JSON`);
		}
		if (!response.ok || data.error) {
			throw new Error(data.error || 'Status request failed');
		}
		errorBox.style.display = 'none';
		alText('al-version', data.version);
		alText('al-mode', data.mode);
		alText('al-interface', data.wan_interface);
		alText('al-updated', data.updated_at);
		alText('al-download', `${data.download.current_mbps.toFixed(1)} / ${data.download.maximum_mbps.toFixed(1)} Mb/s; traffic ${data.download.throughput_mbps.toFixed(1)} Mb/s; pipe ${data.download.pipe}; ${data.download.state}`);
		alText('al-upload', `${data.upload.current_mbps.toFixed(1)} / ${data.upload.maximum_mbps.toFixed(1)} Mb/s; traffic ${data.upload.throughput_mbps.toFixed(1)} Mb/s; pipe ${data.upload.pipe}; ${data.upload.state}`);
		alText('al-latency', `${data.current_rtt_ms.toFixed(2)} ms (${data.delay_delta_ms.toFixed(2)} ms above baseline)`);
		alText('al-reflectors', data.healthy_reflectors);
		alText('al-reason', data.last_reason);
		alText('al-error', data.last_error || '-');
		const reflectorList = document.getElementById('al-reflector-list');
		reflectorList.replaceChildren();
		for (const reflector of (data.reflectors || [])) {
			const row = document.createElement('tr');
			const values = [
				reflector.address,
				reflector.healthy ? 'Healthy' : 'Unavailable',
				`${reflector.current_rtt_ms.toFixed(2)} ms`,
				`${reflector.baseline_rtt_ms.toFixed(2)} ms`,
				`${reflector.delay_delta_ms.toFixed(2)} ms`,
				reflector.last_error || '-'
			];
			for (const value of values) {
				const cell = document.createElement('td');
				cell.textContent = value;
				row.appendChild(cell);
			}
			reflectorList.appendChild(row);
		}
	} catch (error) {
		errorBox.textContent = error.message;
		errorBox.style.display = 'block';
	}
}

refreshAdaptiveLimiter();
setInterval(refreshAdaptiveLimiter, 2000);
</script>

<?php include('foot.inc'); ?>
