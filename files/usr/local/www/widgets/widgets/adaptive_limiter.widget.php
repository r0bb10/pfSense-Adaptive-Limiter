<?php

require_once('guiconfig.inc');
require_once('service-utils.inc');
require_once('util.inc');

const ADAPTIVE_LIMITER_WIDGET_STATUS = '/var/run/adaptive-limiter/status.json';

/*
 * The dashboard defines $widgetkey when including this file. AJAX requests
 * provide it explicitly, so validate it before using it in HTML or selectors.
 */
if (isset($_POST['widgetkey']) || isset($_GET['widgetkey'])) {
	$requested_widgetkey = $_POST['widgetkey'] ?? $_GET['widgetkey'];
	[$widget_name, $widget_id] = array_pad(explode('-', $requested_widgetkey, 2), 2, null);
	if ($widget_name === basename(__FILE__, '.widget.php') && is_numericint($widget_id)) {
		$widgetkey = $requested_widgetkey;
	} else {
		print gettext('Invalid Widget Key');
		exit;
	}
}

if (!isset($widgetkey)) {
	print gettext('Missing Widget Key');
	exit;
}

function adaptive_limiter_widget_escape($value) {
	return htmlspecialchars((string)$value, ENT_QUOTES | ENT_SUBSTITUTE, 'UTF-8');
}

function adaptive_limiter_widget_rate($value) {
	return number_format((float)$value, 1, '.', '') . ' Mb/s';
}

function adaptive_limiter_widget_status() {
	if (!is_readable(ADAPTIVE_LIMITER_WIDGET_STATUS)) {
		return null;
	}
	$data = json_decode(file_get_contents(ADAPTIVE_LIMITER_WIDGET_STATUS), true);
	return is_array($data) ? $data : null;
}

function adaptive_limiter_widget_body() {
	$status = adaptive_limiter_widget_status();
	if ($status === null) {
		return '<tr><td colspan="2" class="text-muted">' .
		    adaptive_limiter_widget_escape(gettext('Status unavailable. The service may be stopped.')) .
		    '</td></tr>';
	}

	$running = is_service_running('adaptive_limiter');
	$service_icon = $running ? 'fa-arrow-up text-success' : 'fa-arrow-down text-danger';
	$service_label = $running ? gettext('Running') : gettext('Stopped');
	$mode = ucfirst($status['mode'] ?? 'unknown');
	$download = $status['download'] ?? [];
	$upload = $status['upload'] ?? [];

	$current_rtt = number_format((float)($status['current_rtt_ms'] ?? 0), 2, '.', '');
	$delta_rtt = number_format((float)($status['delay_delta_ms'] ?? 0), 2, '.', '');
	$last_adjustment = $status['last_adjustment_at'] ?? gettext('No adjustment yet');
	$last_reason = $status['last_reason'] ?? gettext('No decision available');

	$rows = [];
	$rows[] = [gettext('Service'), '<i class="fa-solid ' . $service_icon . '"></i> ' .
	    adaptive_limiter_widget_escape($service_label . ' / ' . $mode)];
	$rows[] = [gettext('Download'), adaptive_limiter_widget_escape(
	    adaptive_limiter_widget_rate($download['current_mbps'] ?? 0) . ' / ' .
	    adaptive_limiter_widget_rate($download['maximum_mbps'] ?? 0) .
	    ' (traffic ' . adaptive_limiter_widget_rate($download['throughput_mbps'] ?? 0) . ')')];
	$rows[] = [gettext('Upload'), adaptive_limiter_widget_escape(
	    adaptive_limiter_widget_rate($upload['current_mbps'] ?? 0) . ' / ' .
	    adaptive_limiter_widget_rate($upload['maximum_mbps'] ?? 0) .
	    ' (traffic ' . adaptive_limiter_widget_rate($upload['throughput_mbps'] ?? 0) . ')')];
	$rows[] = [gettext('Latency'), adaptive_limiter_widget_escape("{$current_rtt} ms (+{$delta_rtt} ms)")];
	$rows[] = [gettext('State'), adaptive_limiter_widget_escape(
	    'DL ' . ($download['state'] ?? 'unknown') . ' / UL ' . ($upload['state'] ?? 'unknown'))];
	$rows[] = [gettext('Last Change'), adaptive_limiter_widget_escape($last_adjustment) . '<br><small>' .
	    adaptive_limiter_widget_escape($last_reason) . '</small>'];

	$html = '';
	foreach ($rows as [$label, $value]) {
		$html .= '<tr><th>' . adaptive_limiter_widget_escape($label) . '</th><td>' . $value . '</td></tr>';
	}
	return $html;
}

if (isset($_POST['ajax'])) {
	print adaptive_limiter_widget_body();
	exit;
}

?>
<div class="table-responsive">
	<table class="table table-striped table-hover table-condensed">
		<tbody id="<?=adaptive_limiter_widget_escape($widgetkey)?>">
			<?=adaptive_limiter_widget_body()?>
		</tbody>
	</table>
</div>
<div class="text-right">
	<a href="/status_adaptive_limiter.php"><?=gettext('Full status')?></a>
</div>

<script type="text/javascript">
events.push(function() {
	function adaptiveLimiterCallback(response) {
		$(<?=json_encode('#' . $widgetkey)?>).html(response);
	}

	var refreshObject = new Object();
	refreshObject.name = 'adaptive-limiter';
	refreshObject.url = '/widgets/widgets/adaptive_limiter.widget.php';
	refreshObject.callback = adaptiveLimiterCallback;
	refreshObject.parms = {
		ajax: 'ajax',
		widgetkey: <?=json_encode($widgetkey)?>
	};
	refreshObject.freq = 2;
	register_ajax(refreshObject);
});
</script>
