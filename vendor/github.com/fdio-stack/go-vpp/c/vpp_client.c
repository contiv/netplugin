/*
 *------------------------------------------------------------------
 * api.c - message handler registration
 *
 * Copyright (c) 2010 Cisco and/or its affiliates.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *------------------------------------------------------------------
 */

#include "vpp_client.h"

#define f64_endian(a)
#define f64_print(a,b)

/* define message structures */
#define vl_typedefs
#include <vpp-api/vpe_all_api_h.h>
#undef vl_typedefs

/* define message structures */
#define vl_endianfun
#include <vpp-api/vpe_all_api_h.h>
#undef vl_endianfun

/* instantiate all the print functions we know about */
#define vl_print(handle, ...)
#define vl_printfun
#include <vpp-api/vpe_all_api_h.h>
#undef vl_printfun

#define vl_api_version(n,v) static u32 api_version=(v);
#include <acl/acl_all_api_h.h>
#undef vl_api_version

/*
 * Satisfy external references when -lvlib is not available.
 */
vlib_main_t vlib_global_main;
vlib_main_t **vlib_mains;

void vlib_cli_output (struct vlib_main_t * vm, char * fmt, ...)
{
  clib_warning ("vlib_cli_output called...");
}

u8 * format_ethernet_address (u8 * s, va_list * args)
{
  u8 * a = va_arg (*args, u8 *);

  return format (s, "%02x:%02x:%02x:%02x:%02x:%02x",
                 a[0], a[1], a[2], a[3], a[4], a[5]);
}

static void vl_api_sw_interface_details_t_handler (
  vl_api_sw_interface_details_t * mp)
{
  char * duplex, * speed;

  switch (mp->link_duplex << VNET_HW_INTERFACE_FLAG_DUPLEX_SHIFT)
  {
  case VNET_HW_INTERFACE_FLAG_HALF_DUPLEX:
    duplex = "half";
    break;
  case VNET_HW_INTERFACE_FLAG_FULL_DUPLEX:
    duplex = "full";
    break;
  default:
    duplex = "bogus";
    break;
  }
  switch (mp->link_speed << VNET_HW_INTERFACE_FLAG_SPEED_SHIFT)
  {
  case VNET_HW_INTERFACE_FLAG_SPEED_10M:
    speed = "10Mbps";
    break;
  case VNET_HW_INTERFACE_FLAG_SPEED_100M:
    speed = "100Mbps";
    break;
  case VNET_HW_INTERFACE_FLAG_SPEED_1G:
    speed = "1Gbps";
    break;
  case VNET_HW_INTERFACE_FLAG_SPEED_10G:
    speed = "10Gbps";
    break;
  case VNET_HW_INTERFACE_FLAG_SPEED_40G:
    speed = "40Gbps";
    break;
  case VNET_HW_INTERFACE_FLAG_SPEED_100G:
    speed = "100Gbps";
    break;
  default:
    speed = "bogus";
    break;
  }
  fformat(stdout, "details: %s sw_if_index %d sup_sw_if_index %d "
          "link_duplex %s link_speed %s",
          mp->interface_name, ntohl(mp->sw_if_index),
          ntohl(mp->sup_sw_if_index), duplex, speed);

  if (mp->l2_address_length)
    fformat(stdout, "  l2 address: %U\n",
            format_ethernet_address, mp->l2_address);
  else
    fformat(stdout, "\n");
}

static void vl_api_sw_interface_set_flags_t_handler (
  vl_api_sw_interface_set_flags_t * mp)
{
  int *sw_if_index = malloc(sizeof(int));

  *sw_if_index = ntohl(mp->sw_if_index);
  fformat (stdout, "set flags: sw_if_index %d, admin %s link %s\n",
           *sw_if_index,
           mp->admin_up_down ? "up" : "down",
           mp->link_up_down ? "up" : "down");

  //alagalah - again GOGC seems to do this ... need to follow up
  //free(sw_if_index);
}

static void vl_api_sw_interface_set_flags_reply_t_handler (
  vl_api_sw_interface_set_flags_reply_t * mp)
{
  gocallback_set_interface_flags(&mp->retval);
}

static void vl_api_want_interface_events_reply_t_handler (
  vl_api_want_interface_events_reply_t *mp)
{
}

static void vl_api_want_stats_reply_t_handler (
  vl_api_want_stats_reply_t *mp)
{
  fformat (stdout, "want stats reply %d\n", ntohl(mp->retval));
}

static void vl_api_ip_add_del_route_reply_t_handler (
  vl_api_ip_add_del_route_reply_t *mp)
{
  fformat (stdout, "add_route reply %d\n", ntohl(mp->retval));
}


static void vl_api_sw_interface_set_table_reply_t_handler (
  vl_api_sw_interface_set_table_reply_t *mp)
{
  fformat (stdout, "set_table reply %d\n", ntohl(mp->retval));
}

static void vl_api_tap_connect_reply_t_handler (
  vl_api_tap_connect_reply_t * mp)
{
  fformat (stdout, "tap connect reply %d, sw_if_index %d\n",
           ntohl(mp->retval), ntohl(mp->sw_if_index));
}

static void vl_api_create_vlan_subif_reply_t_handler (
  vl_api_create_vlan_subif_reply_t * mp)
{
  fformat (stdout, "create vlan subif reply %d, sw_if_index %d\n",
           ntohl(mp->retval), ntohl(mp->sw_if_index));
}

static void vl_api_proxy_arp_add_del_reply_t_handler
(vl_api_proxy_arp_add_del_reply_t *mp)
{
  fformat (stdout, "add del proxy arp reply %d\n", ntohl(mp->retval));
}

static void vl_api_proxy_arp_intfc_enable_disable_reply_t_handler
(vl_api_proxy_arp_intfc_enable_disable_reply_t *mp)
{
  fformat (stdout, "proxy arp intfc ena/dis reply %d\n", ntohl(mp->retval));
}

static void vl_api_vnet_interface_counters_t_handler (
  vl_api_vnet_interface_counters_t *mp)
{
  char *counter_name;
  u32 count, sw_if_index;
  int i;
  int ok = 0; // alagalah temp debugging flag -- remove

  struct timespec timestamp;

  clock_gettime(CLOCK_REALTIME, &timestamp);

  count = ntohl (mp->count);
  sw_if_index = ntohl (mp->first_sw_if_index);
  if (mp->is_combined == 0) {
    u64 * vp, v;
    vp = (u64 *) mp->data;
    vpp_interface_counters_record_t *records , *record;
    records = malloc(sizeof(vpp_interface_counters_record_t));
    records->next = NULL;

    switch (mp->vnet_counter_type) {
    case  VNET_INTERFACE_COUNTER_DROP:
      counter_name = "drop";
      break;
    case  VNET_INTERFACE_COUNTER_PUNT:
      counter_name = "punt";
      break;
    case  VNET_INTERFACE_COUNTER_IP4:
      counter_name = "ip4";
      break;
    case  VNET_INTERFACE_COUNTER_IP6:
      counter_name = "ip6";
      break;
    case  VNET_INTERFACE_COUNTER_RX_NO_BUF:
      counter_name = "rx_no_buf";
      break;
    case  VNET_INTERFACE_COUNTER_RX_MISS:
      counter_name = "rx_miss";
      break;
    case  VNET_INTERFACE_COUNTER_RX_ERROR:
      counter_name = "rx_error";
      break;
    case  VNET_INTERFACE_COUNTER_TX_ERROR:
      counter_name = "tx_error_fifo_full";
      break;
    default:
      counter_name = "bogus";
      break;
    }
    for (i = 0; i < count; i++) {
      v = clib_mem_unaligned (vp, u64);
      v = clib_net_to_host_u64 (v);
      vp++;
      if (ok && v) //alagalah add this to parse out 0 for printing ... but this will go to DB
        fformat (stdout, "%d.%s %lld\n", sw_if_index, counter_name, v);

      record = malloc(sizeof(vpp_interface_counters_record_t));
      record->timestamp = timestamp;
      record->counter_name = counter_name;
      record->sw_if_index = sw_if_index;
      record->counter = v;
      record->next = records;
      records = record;
      sw_if_index++;
    }

    i = sw_if_index - mp->first_sw_if_index ; // vector length
    gocallback_vnet_interface_counters(&i, records);

  } else {
    vlib_counter_t *vp;
    u64 packets, bytes;
    vp = (vlib_counter_t *) mp->data;
    vpp_interface_summary_counters_record_t *summary_records, *summary_record;
    summary_records = malloc(sizeof(vpp_interface_summary_counters_record_t));
    summary_records->next = NULL;


    switch (mp->vnet_counter_type) {
    case  VNET_INTERFACE_COUNTER_RX:
      counter_name = "rx";
      break;
    case  VNET_INTERFACE_COUNTER_TX:
      counter_name = "tx";
      break;
    default:
      counter_name = "bogus";
      break;
    }
    for (i = 0; i < count; i++) {
      packets = clib_mem_unaligned (&vp->packets, u64);
      packets = clib_net_to_host_u64 (packets);
      bytes = clib_mem_unaligned (&vp->bytes, u64);
      bytes = clib_net_to_host_u64 (bytes);
      vp++;
      if (ok && (packets || bytes)) {
        fformat (stdout, "%d.%s.packets %lld\n",
                 sw_if_index, counter_name, packets);
        fformat (stdout, "%d.%s.bytes %lld\n",
                 sw_if_index, counter_name, bytes);
      }

      summary_record = malloc(sizeof(vpp_interface_summary_counters_record_t));
      summary_record->timestamp = timestamp;
      summary_record->counter_name = counter_name;
      summary_record->sw_if_index = sw_if_index;
      summary_record->packet_counter = packets;
      summary_record->byte_counter = bytes;
      summary_record->next = summary_records;
      summary_records = summary_record;
      sw_if_index++;
    }
    i = sw_if_index - mp->first_sw_if_index ; // vector length
    gocallback_vnet_summary_interface_counters(&i, summary_records);
  }

}

/* Format an IP4 address. */
u8 * format_ip4_address (u8 * s, va_list * args)
{
  u8 * a = va_arg (*args, u8 *);
  return format (s, "%d.%d.%d.%d", a[0], a[1], a[2], a[3]);
}

/* Format an IP4 route destination and length. */
u8 * format_ip4_address_and_length (u8 * s, va_list * args)
{
  u8 * a = va_arg (*args, u8 *);
  u8 l = va_arg (*args, u32);
  return format (s, "%U/%d", format_ip4_address, a, l);
}

static void vl_api_vnet_ip4_fib_counters_t_handler (
  vl_api_vnet_ip4_fib_counters_t *mp)
{
  int i;
  vl_api_ip4_fib_counter_t * ctrp;
  u32 count;

  count = ntohl(mp->count);

  fformat (stdout, "fib id %d, count this msg %d\n",
           ntohl(mp->vrf_id), count);

  ctrp = mp->c;
  for (i = 0; i < count; i++) {
    fformat(stdout, "%U: %lld packets, %lld bytes\n",
            format_ip4_address_and_length, &ctrp->address,
            (u32)ctrp->address_length,
            clib_net_to_host_u64 (ctrp->packets),
            clib_net_to_host_u64 (ctrp->bytes));
    ctrp++;
  }
}


/* Format an IP6 address. */
u8 * format_ip6_address (u8 * s, va_list * args)
{
  ip6_address_t * a = va_arg (*args, ip6_address_t *);
  u32 i, i_max_n_zero, max_n_zeros, i_first_zero, n_zeros, last_double_colon;

  i_max_n_zero = ARRAY_LEN (a->as_u16);
  max_n_zeros = 0;
  i_first_zero = i_max_n_zero;
  n_zeros = 0;
  for (i = 0; i < ARRAY_LEN (a->as_u16); i++)
  {
    u32 is_zero = a->as_u16[i] == 0;
    if (is_zero && i_first_zero >= ARRAY_LEN (a->as_u16))
    {
      i_first_zero = i;
      n_zeros = 0;
    }
    n_zeros += is_zero;
    if ((! is_zero && n_zeros > max_n_zeros)
        || (i + 1 >= ARRAY_LEN (a->as_u16) && n_zeros > max_n_zeros))
    {
      i_max_n_zero = i_first_zero;
      max_n_zeros = n_zeros;
      i_first_zero = ARRAY_LEN (a->as_u16);
      n_zeros = 0;
    }
  }

  last_double_colon = 0;
  for (i = 0; i < ARRAY_LEN (a->as_u16); i++)
  {
    if (i == i_max_n_zero && max_n_zeros > 1)
    {
      s = format (s, "::");
      i += max_n_zeros - 1;
      last_double_colon = 1;
    }
    else
    {
      s = format (s, "%s%x",
                  (last_double_colon || i == 0) ? "" : ":",
                  clib_net_to_host_u16 (a->as_u16[i]));
      last_double_colon = 0;
    }
  }

  return s;
}

static void vl_api_reset_fib_reply_t_handler (
  vl_api_reset_fib_reply_t *mp)
{
  fformat(stdout, "fib reset reply %d\n", ntohl(mp->retval));
}

static void vl_api_create_loopback_reply_t_handler
(vl_api_create_loopback_reply_t *mp)
{
  fformat (stdout, "create loopback status %d, sw_if_index %d\n",
           ntohl(mp->retval), ntohl (mp->sw_if_index));
}

static void vl_api_l2_patch_add_del_reply_t_handler
(vl_api_l2_patch_add_del_reply_t *mp)
{
  fformat (stdout, "l2 patch reply %d\n", ntohl(mp->retval));
}

static void vl_api_bridge_domain_dump_t_handler()
{}

static void vl_api_bridge_domain_details_t_handler()
{}

static void vl_api_bridge_domain_sw_if_details_t_handler()
{}

static void vl_api_l2fib_add_del_t_handler()
{}

static void vl_api_sw_interface_add_del_address_reply_t_handler (
  vl_api_sw_interface_add_del_address_reply_t *mp)
{
  fformat (stdout, "add_del_address reply %d\n", ntohl(mp->retval));
  gocallback_add_del_address_reply();
}

static void vl_api_af_packet_create_reply_t_handler (
  vl_api_af_packet_create_reply_t *mp)
{
  int * retval, * sw_if_index;

  retval = malloc(sizeof(int));
  sw_if_index = malloc(sizeof(int));

  *retval = ntohl(mp->retval);
  *sw_if_index = ntohl(mp->sw_if_index);
  //    fformat (stdout, "c: af_packet_create: %d sw_if_index: %d\n", ntohl(mp->retval), ntohl(mp->sw_if_index));

  gocallback_af_packet_create_reply(retval, sw_if_index);
}

/*
------------------ L2 Bridge -------------------------------
*/

static void vl_api_sw_interface_set_l2_bridge_reply_t_handler (
  vl_api_sw_interface_set_l2_bridge_reply_t *mp)
{
  int * retval;
  retval = malloc(sizeof(int));
  *retval = ntohl(mp->retval);

  fformat (stdout, "l2_bridge_set_interface reply %d\n", ntohl(mp->retval));
  gocallback_set_interface_l2_bridge_reply(retval);
}

static void vl_api_bridge_domain_add_del_reply_t_handler (
  vl_api_bridge_domain_add_del_reply_t *mp)
{
  int * retval;
  retval = malloc(sizeof(int));
  *retval = ntohl(mp->retval);

  fformat (stdout, "l2_bridge reply %d\n", ntohl(mp->retval));
  gocallback_add_l2_bridge_reply(retval);
}

/*
------------------ ACL REPLY -------------------------------
*/

static void vl_api_acl_interface_add_del_reply_t_handler (
  vl_api_acl_interface_add_del_reply_t *mp)
{
  int * retval;
  retval = malloc(sizeof(int));
  *retval = ntohl(mp->retval);

  fformat (stdout, "acl_interface_add_del reply %d\n", ntohl(mp->retval));
  gocallback_acl_interface_add_del_reply(retval);
}

static void vl_api_acl_del_reply_t_handler (
  vl_api_acl_del_reply_t *mp)
{
  int * retval;
  retval = malloc(sizeof(int));
  *retval = ntohl(mp->retval);

  fformat (stdout, "acl_del reply %d\n", ntohl(mp->retval));
  gocallback_acl_del_reply(retval);
}

static void vl_api_acl_plugin_get_version_reply_t_handler
(vl_api_acl_plugin_get_version_reply_t * mp)
{
  fformat (stdout, "acl_plugin version: %d.%d", ntohl(mp->major), ntohl(mp->minor));
  gocallback_acl_plugin_get_version(0);
}

/*
-----------------------------------------------------------
*/

// alagalah: see vpp/vpp/vpp-api/summary_stats_client.c
static void
vl_api_vnet_summary_stats_reply_t_handler (
  vl_api_vnet_summary_stats_reply_t * mp)
{
  printf ("total rx pkts %llu, total rx bytes %llu\n",
          (unsigned long long) mp->total_pkts[0],
          (unsigned long long) mp->total_bytes[0]);
  printf ("total tx pkts %llu, total tx bytes %llu\n",
          (unsigned long long) mp->total_pkts[1],
          (unsigned long long) mp->total_bytes[1]);
  printf ("vector rate %.2f\n", mp->vector_rate);

  fformat (stdout, "%.0f,%llu,%llu,%llu,%llu\n%c",
           mp->vector_rate,
           (unsigned long long) mp->total_pkts[0],
           (unsigned long long) mp->total_bytes[0],
           (unsigned long long) mp->total_pkts[1],
           (unsigned long long) mp->total_bytes[1], 0);

}

static void noop_handler (void *notused) { }

#define vl_api_vnet_ip4_fib_counters_t_endian noop_handler
#define vl_api_vnet_ip4_fib_counters_t_print noop_handler
#define vl_api_vnet_ip6_fib_counters_t_endian noop_handler
#define vl_api_vnet_ip6_fib_counters_t_print noop_handler

#define foreach_api_msg                                                 \
_(SW_INTERFACE_DETAILS, sw_interface_details)                           \
_(SW_INTERFACE_SET_FLAGS, sw_interface_set_flags)                       \
_(SW_INTERFACE_SET_FLAGS_REPLY, sw_interface_set_flags_reply)           \
_(WANT_INTERFACE_EVENTS_REPLY, want_interface_events_reply)             \
_(WANT_STATS_REPLY, want_stats_reply)                                   \
_(VNET_INTERFACE_COUNTERS, vnet_interface_counters)                     \
_(VNET_IP4_FIB_COUNTERS, vnet_ip4_fib_counters)                         \
_(IP_ADD_DEL_ROUTE_REPLY, ip_add_del_route_reply)                       \
_(SW_INTERFACE_ADD_DEL_ADDRESS_REPLY, sw_interface_add_del_address_reply) \
_(SW_INTERFACE_SET_TABLE_REPLY, sw_interface_set_table_reply)           \
_(TAP_CONNECT_REPLY, tap_connect_reply)                                 \
_(CREATE_VLAN_SUBIF_REPLY, create_vlan_subif_reply)                     \
_(PROXY_ARP_ADD_DEL_REPLY, proxy_arp_add_del_reply)                     \
_(PROXY_ARP_INTFC_ENABLE_DISABLE_REPLY, proxy_arp_intfc_enable_disable_reply) \
_(RESET_FIB_REPLY, reset_fib_reply)                                     \
_(BRIDGE_DOMAIN_ADD_DEL_REPLY, bridge_domain_add_del_reply)             \
_(AF_PACKET_CREATE_REPLY, af_packet_create_reply)                       \
_(BRIDGE_DOMAIN_DUMP, bridge_domain_dump)                               \
_(BRIDGE_DOMAIN_DETAILS, bridge_domain_details)                         \
_(BRIDGE_DOMAIN_SW_IF_DETAILS, bridge_domain_sw_if_details)             \
_(L2FIB_ADD_DEL, l2fib_add_del)                                         \
_(CREATE_LOOPBACK_REPLY, create_loopback_reply)                         \
_(L2_PATCH_ADD_DEL_REPLY, l2_patch_add_del_reply)                       \
_(SW_INTERFACE_SET_L2_BRIDGE_REPLY, sw_interface_set_l2_bridge_reply)   \
_(VNET_SUMMARY_STATS_REPLY, vnet_summary_stats_reply)

#define foreach_vpe_api_reply_msg                                       \
_(ACL_DEL_REPLY, acl_del_reply)                                         \
_(ACL_INTERFACE_ADD_DEL_REPLY, acl_interface_add_del_reply)             \
_(ACL_PLUGIN_GET_VERSION_REPLY, acl_plugin_get_version_reply)

int connect_to_vpp(client_main_t *cm)
{

  int rv = 0;
  rv = vl_client_connect_to_vlib("/vpe-api", cm->my_client_name, 32);
#define _(N,n)                                                  \
      vl_msg_api_set_handlers((VL_API_##N),                       \
                             #n,                                  \
                             vl_api_##n##_t_handler,              \
                             noop_handler,                        \
                             vl_api_##n##_t_endian,               \
                             vl_api_##n##_t_print,                \
                             sizeof(vl_api_##n##_t), 1);
  foreach_api_msg;
#undef _
  u8 * name;

  name = format (0, "acl_%08x%c", api_version, 0);
  cm->msg_id_base = vl_client_get_first_plugin_msg_id ((char *) name);

#define _(N,n)                                                \
    vl_msg_api_set_handlers((VL_API_##N + cm->msg_id_base),     \
                           #n,                                  \
                           vl_api_##n##_t_handler,              \
                           vl_noop_handler,                     \
                           vl_api_##n##_t_endian,               \
                           vl_api_##n##_t_print,                \
                           sizeof(vl_api_##n##_t), 1);
  foreach_vpe_api_reply_msg;
#undef _
  cm->vl_input_queue = api_main.shmem_hdr->vl_input_queue;
  cm->my_client_index = api_main.my_client_index;
  gocallback_connect_to_vpp(cm);
  return rv;
}

int disconnect_from_vpp(void)
{
  vl_client_disconnect_from_vlib();
  return 0;
}

void link_up_down_enable_disable (client_main_t *tm, int enable)
{
  vl_api_want_interface_events_t * mp;

  /* Request admin / link up down messages */
  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_WANT_INTERFACE_EVENTS);
  mp->client_index = tm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->enable_disable = enable;
  mp->pid = getpid();
  vl_msg_api_send_shmem (tm->vl_input_queue, (u8 *)&mp);
  tm->link_events_on = enable;
}

void add_del_ip4_route (client_main_t *tm, int enable_disable)
{
  vl_api_ip_add_del_route_t *mp;
  u32 tmp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_IP_ADD_DEL_ROUTE);
  mp->client_index = tm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->table_id = ntohl(0);
  mp->create_vrf_if_needed = 1;
  /* Arp, please, if needed */
  // mp->resolve_if_needed = 1;
  // mp->resolve_attempts = ntohl(10);

  mp->next_hop_sw_if_index = ntohl(5);
  mp->is_add = enable_disable;
  mp->next_hop_weight = 1;

  /* Next hop: 6.0.0.1 */
  tmp = ntohl(0x06000001);
  clib_memcpy (mp->next_hop_address, &tmp, sizeof (tmp));

  /* Destination: 10.0.0.1/32 */
  tmp = ntohl(0x0);
  clib_memcpy (mp->dst_address, &tmp, sizeof (tmp));
  mp->dst_address_length = 0;

  vl_msg_api_send_shmem (tm->vl_input_queue, (u8 *)&mp);
}


void del_all_interface_addresses (client_main_t *tm)
{
  vl_api_sw_interface_add_del_address_t *mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_SW_INTERFACE_ADD_DEL_ADDRESS);
  mp->client_index = tm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->sw_if_index = ntohl(5);
  mp->del_all = 1;

  vl_msg_api_send_shmem (tm->vl_input_queue, (u8 *)&mp);
}

void set_interface_table (client_main_t *tm, int is_ipv6, u32 vrf_id)
{
  vl_api_sw_interface_set_table_t *mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_SW_INTERFACE_SET_TABLE);
  mp->client_index = tm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->sw_if_index = ntohl(5);
  mp->is_ipv6 = is_ipv6;
  mp->vrf_id = ntohl(vrf_id);

  vl_msg_api_send_shmem (tm->vl_input_queue, (u8 *)&mp);
}

void create_vlan_subif (client_main_t *tm, u32 vlan_id)
{
  vl_api_create_vlan_subif_t *mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_CREATE_VLAN_SUBIF);
  mp->client_index = tm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->sw_if_index = ntohl (5);
  mp->vlan_id = ntohl(vlan_id);

  vl_msg_api_send_shmem (tm->vl_input_queue, (u8 *)&mp);
}

void add_del_proxy_arp (client_main_t *tm, int is_add)
{
  vl_api_proxy_arp_add_del_t *mp;
  u32 tmp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_PROXY_ARP_ADD_DEL);
  mp->client_index = tm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->vrf_id = ntohl(11);
  mp->is_add = is_add;

  /* proxy fib 11, 1.1.1.1 -> 1.1.1.10 */
  tmp = ntohl (0x01010101);
  clib_memcpy (mp->low_address, &tmp, 4);

  tmp = ntohl (0x0101010a);
  clib_memcpy (mp->hi_address, &tmp, 4);

  vl_msg_api_send_shmem (tm->vl_input_queue, (u8 *)&mp);
}

void add_ip4_neighbor (client_main_t *tm, int add_del)
{
  vl_api_ip_neighbor_add_del_t *mp;
  u32 tmp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_IP_NEIGHBOR_ADD_DEL);
  mp->client_index = tm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->vrf_id = ntohl(11);
  mp->sw_if_index = ntohl(6);
  mp->is_add = add_del;

  memset (mp->mac_address, 0xbe, sizeof (mp->mac_address));

  tmp = ntohl (0x0101010a);
  clib_memcpy (mp->dst_address, &tmp, 4);

  vl_msg_api_send_shmem (tm->vl_input_queue, (u8 *)&mp);
}

void reset_fib (client_main_t *tm, u8 is_ip6)
{
  vl_api_reset_fib_t *mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_RESET_FIB);
  mp->client_index = tm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->vrf_id = ntohl(11);
  mp->is_ipv6 = is_ip6;

  vl_msg_api_send_shmem (tm->vl_input_queue, (u8 *)&mp);
}

/*
------------------ Interfaces -------------------------------
*/

void loop_create (client_main_t *tm)
{
  vl_api_create_loopback_t * mp;

  mp = vl_msg_api_alloc (sizeof(*mp));
  memset(mp, 0, sizeof (*mp));

  mp->_vl_msg_id = ntohs (VL_API_CREATE_LOOPBACK);
  mp->client_index = tm->my_client_index;
  mp->context = 0xdeadbeef;
  vl_msg_api_send_shmem (tm->vl_input_queue, (u8 *)&mp);
}

void add_del_interface_address (int enable_disable, int *sw_if_index, u32 *ipaddr, u8 *length, client_main_t *cm)
{
  vl_api_sw_interface_add_del_address_t *mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_SW_INTERFACE_ADD_DEL_ADDRESS);
  mp->client_index = cm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->sw_if_index = ntohl(*sw_if_index);
  mp->is_add = enable_disable;
  mp->address_length = *length;

  *ipaddr = ntohl (*ipaddr);
  clib_memcpy (mp->address, ipaddr, 4);

  vl_msg_api_send_shmem (cm->vl_input_queue, (u8 *)&mp);
}

void stats_enable_disable (int enable, client_main_t *cm)
{
  vl_api_want_stats_t * mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_WANT_STATS);
  mp->client_index = cm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->enable_disable = enable;
  mp->pid = getpid();
  vl_msg_api_send_shmem (cm->vl_input_queue, (u8 *)&mp);
  cm->stats_on = enable;
}

void set_flags (int *sw_if_index, int *up_down, client_main_t *cm)
{
  vl_api_sw_interface_set_flags_t * mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));

  mp->_vl_msg_id = ntohs (VL_API_SW_INTERFACE_SET_FLAGS);
  mp->client_index = cm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->sw_if_index = ntohl (*sw_if_index);
  mp->admin_up_down = *up_down;
  vl_msg_api_send_shmem (cm->vl_input_queue, (u8 *)&mp);
  fformat(stdout, "Hello from set_flags\n");
}


void get_vpp_summary_stats(client_main_t *cm)
{
  vl_api_vnet_get_summary_stats_t * mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_VNET_GET_SUMMARY_STATS);
  mp->client_index = cm->my_client_index;
  vl_msg_api_send_shmem (cm->vl_input_queue, (u8 *)&mp);
}

void add_af_packet_interface(char* intf, client_main_t *cm)
{
  vl_api_af_packet_create_t *mp;
  u8 hw_addr[6];
  u8 random_hw_addr = 1;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_AF_PACKET_CREATE);
  mp->client_index = cm->my_client_index;
  mp->context = 0xdeadbeef;
  // <<<<

  memset (hw_addr, 0, sizeof (hw_addr));
  clib_memcpy (mp->host_if_name, intf, sizeof(char) * strlen(intf));
  clib_memcpy (mp->hw_addr, hw_addr, 6);
  mp->use_random_hw_addr = random_hw_addr;

  //    fformat(stdout,"c: sending create_af_packet_interface for %s\n", intf);
  vl_msg_api_send_shmem (cm->vl_input_queue, (u8 *)&mp);
}

/*
------------------ L2 -------------------------------------
*/

void l2_patch_add_del (client_main_t *tm, int is_add)
{
  vl_api_l2_patch_add_del_t *mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_L2_PATCH_ADD_DEL);
  mp->client_index = tm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->is_add = is_add;
  mp->rx_sw_if_index = ntohl (1);
  mp->tx_sw_if_index = ntohl (2);

  vl_msg_api_send_shmem (tm->vl_input_queue, (u8 *)&mp);
}

void add_l2_bridge (int bd_id, client_main_t *cm)
{
  //TODO Check if bridge exists.
  //TODO Take a name for bridge and keep map ?
  vl_api_bridge_domain_add_del_t *mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_BRIDGE_DOMAIN_ADD_DEL);
  mp->client_index = cm->my_client_index;
  mp->context = 0xdeadbeef;

  mp->bd_id = ntohl(bd_id);
  mp->flood = 1;
  mp->uu_flood = 1;
  mp->forward = 1;
  mp->learn = 1;
  mp->arp_term = 0 ;
  mp->is_add = 1;

  fformat(stdout, "c: sending add_l2_bridge req. to vpp\n");
  vl_msg_api_send_shmem (cm->vl_input_queue, (u8 *)&mp);
}

void set_interface_l2_bridge (int bd_id, int *rx_if_index, client_main_t *cm)
{
  vl_api_sw_interface_set_l2_bridge_t *mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_SW_INTERFACE_SET_L2_BRIDGE);
  mp->client_index = cm->my_client_index;
  mp->context = 0xdeadbeef;

  mp->bd_id = ntohl(bd_id);
  mp->rx_sw_if_index = ntohl (*rx_if_index);
  mp->shg = 0;
  mp->bvi = 0 ;
  mp->enable = 1;

  //    fformat(stdout,"c: sending add_l2_bridge_interface req. to vpp\n");
  vl_msg_api_send_shmem (cm->vl_input_queue, (u8 *)&mp);
}

/*
------------------ ACL ------------------------------------
*/

void dump_acl (int aclIndex, client_main_t *cm)
{
  fformat(stdout, "Hello from dump_acl\n");
  vl_api_acl_dump_t *mp;
  u8 * name;

  name = format (0, "acl_%08x%c", api_version, 0);
  cm->msg_id_base = vl_client_get_first_plugin_msg_id ((char *) name);

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_ACL_DUMP);
  mp->client_index = cm->my_client_index;
  mp->context = 0xdeadbeef;

  mp->acl_index = ntohl(aclIndex);

  //    fformat(stdout,"c: sending add_l2_bridge_interface req. to vpp\n");
  vl_msg_api_send_shmem (cm->vl_input_queue, (u8 *)&mp);
}

void acl_del (int aclIndex, client_main_t *cm)
{
  fformat(stdout, "Hello from acl_del\n");
  vl_api_acl_del_t *mp;
  u8 * name;

  name = format (0, "acl_%08x%c", api_version, 0);
  cm->msg_id_base = vl_client_get_first_plugin_msg_id ((char *) name);

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));

  //mp->_vl_msg_id = ntohs (VL_API_ACL_DEL);
  mp->_vl_msg_id = ntohs (VL_API_ACL_DEL + cm->msg_id_base);
  mp->client_index = cm->my_client_index;
  mp->context = 0xdeadbeef;
  mp->acl_index = ntohl(aclIndex);

  fformat(stdout, "c: sending acl_del req. to vpp\n");
  fformat(stdout, "c: sending acl_del req. to vpp with plugin_id:%d\n", cm->msg_id_base);
  fformat(stdout, "c: sending acl_del req. to vpp with msg_id:%d\n", mp->_vl_msg_id);
  vl_msg_api_send_shmem (cm->vl_input_queue, (u8 *)&mp);
}

void acl_interface_add_del (int isAdd, int isInput, int *sw_if_index, int aclIndex, client_main_t *cm)
{
  fformat(stdout, "Hello from acl_interface_add_del\n");
  vl_api_acl_interface_add_del_t *mp;

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));

  mp->_vl_msg_id = ntohs (VL_API_ACL_INTERFACE_ADD_DEL + cm->msg_id_base);
  mp->client_index = cm->my_client_index;
  mp->context = 0xdeadbeef;

  mp->is_add = isAdd;
  mp->is_input = isInput;
  mp->acl_index = ntohl(aclIndex);
  mp->sw_if_index = ntohl(*sw_if_index);
  fformat(stdout, "c: sending acl_interface_add_del req. to vpp with id:%d\n", mp->_vl_msg_id);

  //fformat(stdout,"c: sending acl_interface_add_del req. to vpp\n");
  vl_msg_api_send_shmem (cm->vl_input_queue, (u8 *)&mp);
}

void acl_plugin_get_version(client_main_t *cm)
{
  fformat(stdout, "Hello from acl_plugin_get_version\n");
  vl_api_acl_plugin_get_version_t *mp;
  u8 * name;

  name = format (0, "acl_%08x%c", api_version, 0);
  cm->msg_id_base = vl_client_get_first_plugin_msg_id ((char *) name);

  mp = vl_msg_api_alloc (sizeof (*mp));
  memset(mp, 0, sizeof (*mp));
  mp->_vl_msg_id = ntohs (VL_API_ACL_PLUGIN_GET_VERSION + cm->msg_id_base);
  mp->client_index = cm->my_client_index;
  mp->context = 0xdeadbeef;

  //    fformat(stdout,"c: sending add_l2_bridge_interface req. to vpp\n");
  vl_msg_api_send_shmem (cm->vl_input_queue, (u8 *)&mp);
}


/*
-----------------------------------------------------------
*/


#undef vl_api_version
#define vl_api_version(n,v) static u32 vpe_api_version = v;
#include <vpp-api/vpe.api.h>
#undef vl_api_version

void vl_client_add_api_signatures (vl_api_memclnt_create_t *mp)
{
  /*
   * Send the main API signature in slot 0. This bit of code must
   * match the checks in ../vpe/api/api.c: vl_msg_api_version_check().
   */
  mp->api_versions[0] = clib_host_to_net_u32 (vpe_api_version);
}