<?php

global $priv_list;

$priv_list['page-services-adaptive-limiter'] = [];
$priv_list['page-services-adaptive-limiter']['name'] = 'WebCfg - Services: Adaptive Limiter';
$priv_list['page-services-adaptive-limiter']['descr'] = 'Allow access to Adaptive Limiter settings and status';
$priv_list['page-services-adaptive-limiter']['match'] = [];
$priv_list['page-services-adaptive-limiter']['match'][] = 'pkg_edit.php?xml=adaptive-limiter.xml*';
$priv_list['page-services-adaptive-limiter']['match'][] = 'status_adaptive_limiter.php*';

?>
