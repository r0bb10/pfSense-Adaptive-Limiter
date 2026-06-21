<?php

require_once("functions.inc");
require_once("globals.inc");
require_once("interfaces.inc");
require_once("service-utils.inc");
require_once("util.inc");

const ADAPTIVE_LIMITER_CONFIG_DIR = '/usr/local/etc/adaptive-limiter';
const ADAPTIVE_LIMITER_CONFIG = ADAPTIVE_LIMITER_CONFIG_DIR . '/config.json';
const ADAPTIVE_LIMITER_RCCONF = '/usr/local/etc/rc.conf.d/adaptive_limiter';
const ADAPTIVE_LIMITER_STATUS = '/var/run/adaptive-limiter/status.json';

function adaptive_limiter_settings() {
	return config_get_path('installedpackages/adaptivelimiter/config/0', []);
}

function adaptive_limiter_real_interface($interface) {
	if (function_exists('get_real_interface')) {
		$real_interface = get_real_interface($interface);
		if (!empty($real_interface)) {
			return $real_interface;
		}
	}
	return $interface;
}

function adaptive_limiter_number($value) {
	return is_numeric($value) ? (float)$value : null;
}

function adaptive_limiter_validate_direction($name, $post, &$input_errors) {
	$prefix = strtolower($name);
	$pipe = $post["{$prefix}_pipe"] ?? '';
	$minimum = adaptive_limiter_number($post["{$prefix}_minimum"] ?? null);
	$baseline = adaptive_limiter_number($post["{$prefix}_baseline"] ?? null);
	$maximum = adaptive_limiter_number($post["{$prefix}_maximum"] ?? null);

	if (!ctype_digit((string)$pipe) || (int)$pipe <= 0) {
		$input_errors[] = gettext("{$name} pipe must be a positive integer.");
	}
	if ($minimum === null || $baseline === null || $maximum === null ||
	    $minimum <= 0 || $minimum > $baseline || $baseline > $maximum) {
		$input_errors[] = gettext("{$name} rates must satisfy 0 < minimum <= baseline <= maximum.");
	}
}

function adaptive_limiter_parse_reflectors($value) {
	return array_values(array_filter(array_map('trim', explode(',', (string)$value))));
}

function adaptive_limiter_validate(&$post, &$input_errors) {
	$interface = $post['wan_interface'] ?? '';
	if (!preg_match('/^[A-Za-z0-9_.:-]+$/', $interface)) {
		$input_errors[] = gettext('WAN interface contains invalid characters.');
	}

	adaptive_limiter_validate_direction('Download', $post, $input_errors);
	adaptive_limiter_validate_direction('Upload', $post, $input_errors);
	if (($post['download_pipe'] ?? '') === ($post['upload_pipe'] ?? '')) {
		$input_errors[] = gettext('Download and upload pipes must differ.');
	}

	$reflectors = adaptive_limiter_parse_reflectors($post['reflectors'] ?? '');
	if (count($reflectors) === 0) {
		$input_errors[] = gettext('At least one reflector is required.');
	}
	foreach ($reflectors as $reflector) {
		if (!is_ipaddrv4($reflector)) {
			$input_errors[] = sprintf(gettext('%s is not a valid IPv4 reflector address.'), $reflector);
		}
	}

	$threshold = adaptive_limiter_number($post['latency_threshold'] ?? null);
	if ($threshold === null || $threshold <= 0 || $threshold > 1000) {
		$input_errors[] = gettext('Latency threshold must be greater than 0 and no more than 1000 ms.');
	}
	foreach (['sample_interval', 'probe_interval', 'probe_timeout', 'adjustment_delay'] as $field) {
		if (!preg_match('/^[0-9]+(?:ms|s|m)$/', $post[$field] ?? '')) {
			$input_errors[] = sprintf(gettext('%s must be a duration such as 500ms, 1s, or 2m.'), $field);
		}
	}
	$sample_ms = adaptive_limiter_duration_ms($post['sample_interval'] ?? '');
	$probe_ms = adaptive_limiter_duration_ms($post['probe_interval'] ?? '');
	$timeout_ms = adaptive_limiter_duration_ms($post['probe_timeout'] ?? '');
	$adjustment_ms = adaptive_limiter_duration_ms($post['adjustment_delay'] ?? '');
	if ($sample_ms !== null && ($sample_ms < 200 || $sample_ms > 60000)) {
		$input_errors[] = gettext('Sample interval must be between 200ms and 1m.');
	}
	if ($probe_ms !== null && ($probe_ms < 100 || $probe_ms > 10000)) {
		$input_errors[] = gettext('Probe interval must be between 100ms and 10s.');
	}
	if ($timeout_ms !== null && ($timeout_ms < 100 || $timeout_ms > 10000)) {
		$input_errors[] = gettext('Probe timeout must be between 100ms and 10s.');
	}
	if ($sample_ms !== null && $adjustment_ms !== null &&
	    ($adjustment_ms < $sample_ms || $adjustment_ms > 600000)) {
		$input_errors[] = gettext('Adjustment delay must be at least the sample interval and no more than 10m.');
	}
}

function adaptive_limiter_duration_ms($value) {
	if (!preg_match('/^([0-9]+)(ms|s|m)$/', (string)$value, $matches)) {
		return null;
	}
	$multiplier = ['ms' => 1, 's' => 1000, 'm' => 60000][$matches[2]];
	return (int)$matches[1] * $multiplier;
}

function adaptive_limiter_daemon_config() {
	$settings = adaptive_limiter_settings();
	$wan_interface = $settings['wan_interface'] ?? 'wan';
	return [
		'enabled' => (($settings['enable'] ?? '') === 'on'),
		'monitor_only' => (($settings['monitor_only'] ?? 'on') === 'on'),
		'wan_interface' => adaptive_limiter_real_interface($wan_interface),
		'download' => [
			'pipe' => (int)($settings['download_pipe'] ?? 1),
			'minimum_mbps' => (float)($settings['download_minimum'] ?? 500),
			'baseline_mbps' => (float)($settings['download_baseline'] ?? 1800),
			'maximum_mbps' => (float)($settings['download_maximum'] ?? 2500),
		],
		'upload' => [
			'pipe' => (int)($settings['upload_pipe'] ?? 2),
			'minimum_mbps' => (float)($settings['upload_minimum'] ?? 250),
			'baseline_mbps' => (float)($settings['upload_baseline'] ?? 800),
			'maximum_mbps' => (float)($settings['upload_maximum'] ?? 1000),
		],
		'reflectors' => adaptive_limiter_parse_reflectors($settings['reflectors'] ?? '1.1.1.1,1.0.0.1,9.9.9.9,8.8.8.8'),
		'latency_threshold_ms' => (float)($settings['latency_threshold'] ?? 15),
		'sample_interval' => $settings['sample_interval'] ?? '1s',
		'probe_interval' => $settings['probe_interval'] ?? '250ms',
		'probe_timeout' => $settings['probe_timeout'] ?? '1s',
		'adjustment_delay' => $settings['adjustment_delay'] ?? '2s',
		'status_path' => ADAPTIVE_LIMITER_STATUS,
	];
}

function adaptive_limiter_write_files() {
	if (!is_dir(ADAPTIVE_LIMITER_CONFIG_DIR)) {
		mkdir(ADAPTIVE_LIMITER_CONFIG_DIR, 0755, true);
	}
	$config = json_encode(adaptive_limiter_daemon_config(), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . "\n";
	$tmp = ADAPTIVE_LIMITER_CONFIG . '.tmp';
	file_put_contents($tmp, $config, LOCK_EX);
	chmod($tmp, 0600);
	rename($tmp, ADAPTIVE_LIMITER_CONFIG);

	if (!is_dir(dirname(ADAPTIVE_LIMITER_RCCONF))) {
		mkdir(dirname(ADAPTIVE_LIMITER_RCCONF), 0755, true);
	}
	$enabled = ((adaptive_limiter_settings()['enable'] ?? '') === 'on') ? 'YES' : 'NO';
	file_put_contents(ADAPTIVE_LIMITER_RCCONF, "adaptive_limiter_enable=\"{$enabled}\"\n", LOCK_EX);
}

function adaptive_limiter_sync() {
	adaptive_limiter_write_files();
	$enabled = ((adaptive_limiter_settings()['enable'] ?? '') === 'on');
	if ($enabled) {
		if (is_service_running('adaptive_limiter')) {
			restart_service('adaptive_limiter');
		} else {
			start_service('adaptive_limiter');
		}
	} elseif (is_service_running('adaptive_limiter')) {
		stop_service('adaptive_limiter');
	}
}

function adaptive_limiter_install() {
	adaptive_limiter_write_files();
}

function adaptive_limiter_deinstall() {
	if (is_service_running('adaptive_limiter')) {
		stop_service('adaptive_limiter');
	}
	@unlink(ADAPTIVE_LIMITER_CONFIG);
	@unlink(ADAPTIVE_LIMITER_RCCONF);
	@unlink(ADAPTIVE_LIMITER_STATUS);
}

?>
