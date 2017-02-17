#include <stdio.h>
#include <stdlib.h>
#include <sys/types.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <netinet/in.h>
#include <signal.h>
#include <pthread.h>
#include <unistd.h>
#include <time.h>
#include <fcntl.h>
#include <string.h>
#include <arpa/inet.h>

#include <vppinfra/clib.h>
#include <vppinfra/vec.h>
#include <vppinfra/hash.h>
#include <vppinfra/bitmap.h>
#include <vppinfra/fifo.h>
#include <vppinfra/time.h>
#include <vppinfra/mheap.h>
#include <vppinfra/heap.h>
#include <vppinfra/pool.h>
#include <vppinfra/format.h>
#include <vppinfra/error.h>

#include <vnet/vnet.h>
#include <vlib/vlib.h>
#include <vlib/unix/unix.h>
#include <vlibapi/api.h>
#include <vlibmemory/api.h>

#include <vpp-api/vpe_msg_enum.h>

#define vl_msg_id_t
#define VL_MSG_FIRST_AVAILABLE
#include <acl/acl_msg_enum.h>
#undef VL_MSG_FIRST_AVAILABLE
#undef vl_msg_id_t

/* define message structures */
#define vl_typedefs
#include <acl/acl_all_api_h.h>
#undef vl_typedefs

/* define generated endian-swappers */
#define vl_endianfun
#include <acl/acl_all_api_h.h>
#undef vl_endianfun

/* instantiate all the print functions we know about */
#define vl_print(handle, ...)
#define vl_printfun
#include <acl/acl_all_api_h.h>
#undef vl_printfun


#include <vnet/ip/ip.h>
#include <vnet/interface.h>

typedef struct {
    int link_events_on;
    int stats_on;
    int oam_events_on;
    /* temporary parse buffer */
    unformat_input_t *input;
    /* convenience */
    unix_shared_memory_queue_t * vl_input_queue;
    u32 my_client_index;
    u16 msg_id_base;
    char *my_client_name;
} client_main_t;

typedef struct vpp_interface_counters_record {
    struct timespec timestamp;
    int sw_if_index;
    char* counter_name;
    u64 counter;
    struct vpp_interface_counters_record *next;
} vpp_interface_counters_record_t;

typedef struct vpp_interface_summary_counters_record {
    struct timespec timestamp;
    int sw_if_index;
    char* counter_name;
    u64 packet_counter;
    u64 byte_counter;
    struct vpp_interface_summary_counters_record *next;
} vpp_interface_summary_counters_record_t;

client_main_t cm;

/* VPP connection */
int connect_to_vpp(client_main_t *cm);
int disconnect_from_vpp(void);
/* Interfaces */
void set_flags (int *sw_if_index, int *up_down, client_main_t *cm);
void add_af_packet_interface (char *intf, client_main_t *cm);
void add_del_interface_address (int enable_disable, int *sw_if_index, u32 *ipaddr, u8 *length, client_main_t *cm);
/* L2 */
void l2_patch_add_del (client_main_t *tm, int is_add);
void add_l2_bridge (int bd_id, client_main_t *cm);
void set_interface_l2_bridge (int bd_id, int *rx_if_index, client_main_t *cm);
/* Stats */
void stats_enable_disable (int enable, client_main_t *cm);
void get_vpp_summary_stats(client_main_t *cm);
/* ACL */
void dump_acl (int aclIndex, client_main_t *cm);
void acl_del (int aclIndex, client_main_t *cm);
void acl_interface_add_del (int isAdd, int isInput, int *sw_if_index, int aclIndex, client_main_t *cm);
void acl_plugin_get_version(client_main_t *cm);

/* Callbacks to GO functions - must have //export Gocallback_ above GO func declation */
/* VPP connection */
extern void gocallback_connect_to_vpp (client_main_t *cm);
/* Interfaces */
extern void gocallback_af_packet_create_reply (int *retval, int *sw_if_index);
extern void gocallback_add_del_address_reply ();
extern void gocallback_set_interface_flags (int *retval);
/* L2 */
extern void gocallback_add_l2_bridge_reply (int *retval);
extern void gocallback_set_interface_l2_bridge_reply (int *retval);
/* Stats */
extern void gocallback_vnet_summary_interface_counters (int *record_count, vpp_interface_summary_counters_record_t *records);
extern void gocallback_vnet_interface_counters (int *record_count, vpp_interface_counters_record_t *records);
/* ACL */
extern void gocallback_acl_interface_add_del_reply (int *retval);
extern void gocallback_acl_del_reply (int *retval);
extern void gocallback_acl_plugin_get_version(int *retval);
