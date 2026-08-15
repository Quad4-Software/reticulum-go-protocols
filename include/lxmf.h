#ifndef LXMF_H
#define LXMF_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define LXMF_API_VERSION "1.0"
#define LXMF_HASH_LEN 16

#define LXMF_OK 0
#define LXMF_ERR_INVALID_ARG 1
#define LXMF_ERR_INVALID_HANDLE 2
#define LXMF_ERR_INTERNAL 6
#define LXMF_ERR_TRUNCATED 8

const char *lxmf_version(void);
int lxmf_last_error(char *buf, size_t buf_len, size_t *written);

uint64_t lxmf_identity_generate(void);
int lxmf_identity_destroy(uint64_t identity);
int lxmf_identity_hash(uint64_t identity, uint8_t *out, size_t out_len, size_t *written);
int lxmf_identity_public_key(uint64_t identity, uint8_t *out, size_t out_len, size_t *written);
int lxmf_identity_delivery_hash(uint64_t identity, uint8_t *out, size_t out_len, size_t *written);
int lxmf_identity_register_recall(uint64_t identity);
int lxmf_identity_register_recall_source(const uint8_t *source, size_t source_len,
	const uint8_t *public_key, size_t public_key_len);

uint64_t lxmf_message_create(const uint8_t *dest, size_t dest_len,
	const uint8_t *source, size_t source_len,
	const char *title, const char *content);
int lxmf_message_pack(uint64_t message, uint64_t identity,
	uint8_t *out, size_t out_len, size_t *written);
uint64_t lxmf_message_unpack(const uint8_t *data, size_t data_len);
int lxmf_message_get_dest(uint64_t message, uint8_t *out, size_t out_len, size_t *written);
int lxmf_message_get_source(uint64_t message, uint8_t *out, size_t out_len, size_t *written);
int lxmf_message_get_title(uint64_t message, char *buf, size_t buf_len, size_t *written);
int lxmf_message_get_content(uint64_t message, char *buf, size_t buf_len, size_t *written);
int lxmf_message_set_fields_json(uint64_t message, const char *json);
int lxmf_message_fields_json(uint64_t message, char *buf, size_t buf_len, size_t *written);
int lxmf_message_field_count(uint64_t message, size_t *count);
uint64_t lxmf_message_unpack_verified(const uint8_t *data, size_t data_len, uint64_t identity);
int lxmf_message_destroy(uint64_t message);

#ifdef __cplusplus
}
#endif

#endif
