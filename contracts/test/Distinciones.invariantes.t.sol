// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import {CommonBase} from "forge-std/Base.sol";
import {StdCheats} from "forge-std/StdCheats.sol";
import {StdUtils} from "forge-std/StdUtils.sol";

import {Base} from "./Base.t.sol";
import {Distinciones} from "../src/Distinciones.sol";

/// @dev El que sacude el contrato: acuña, y después prueba de todas las formas
///      que existen de mover un token. Anota aparte quién es el dueño de cada
///      id para que la invariante tenga con qué comparar.
contract Sacudidor is CommonBase, StdCheats, StdUtils {
    Distinciones public immutable distinciones;
    uint256 private immutable pkFirmante;

    uint256[] public ids;
    mapping(uint256 id => address duenio) public duenioDe;
    address[] public duenios;
    mapping(address duenio => uint256 cuantas) public tieneQue;

    uint256 public acunadas;
    uint256 public intentosDeMover;

    constructor(Distinciones _distinciones, uint256 _pkFirmante) {
        distinciones = _distinciones;
        pkFirmante = _pkFirmante;
    }

    function cantidadIds() external view returns (uint256) {
        return ids.length;
    }

    function cantidadDuenios() external view returns (uint256) {
        return duenios.length;
    }

    function acunar(uint256 semilla, uint16 pieza) external {
        address to = address(uint160(bound(semilla, 1, type(uint160).max)));
        pieza = uint16(bound(pieza, 0, distinciones.cantidadPiezas() - 1));
        uint64 deadline = uint64(block.timestamp + 1 hours);

        uint256 id = distinciones.idDe(to, pieza);
        if (duenioDe[id] != address(0)) return;

        bytes32 digest = _digest(to, pieza, deadline);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(pkFirmante, digest);

        distinciones.mint(to, pieza, deadline, abi.encodePacked(r, s, v));

        ids.push(id);
        duenioDe[id] = to;
        if (tieneQue[to] == 0) duenios.push(to);
        tieneQue[to] += 1;
        acunadas += 1;
    }

    function intentarTransferir(uint256 i, address destino) external {
        if (ids.length == 0) return;
        uint256 id = ids[bound(i, 0, ids.length - 1)];
        address de = duenioDe[id];

        intentosDeMover += 1;
        vm.prank(de);
        try distinciones.transferFrom(de, destino, id) {} catch {}
    }

    function intentarSafeTransferir(uint256 i, address destino) external {
        if (ids.length == 0) return;
        uint256 id = ids[bound(i, 0, ids.length - 1)];
        address de = duenioDe[id];

        intentosDeMover += 1;
        vm.prank(de);
        try distinciones.safeTransferFrom(de, destino, id) {} catch {}
    }

    /// @dev El camino largo: aprobar a un tercero y que el tercero lo mueva.
    function intentarAprobarYTransferir(uint256 i, address operador) external {
        if (ids.length == 0) return;
        uint256 id = ids[bound(i, 0, ids.length - 1)];
        address de = duenioDe[id];
        if (operador == de || operador == address(0)) return;

        vm.prank(de);
        try distinciones.setApprovalForAll(operador, true) {} catch {}

        intentosDeMover += 1;
        vm.prank(operador);
        try distinciones.transferFrom(de, operador, id) {} catch {}
    }

    /// @dev Volver a usar un voucher ya usado. Como el id sale de la dirección,
    ///      reclamar "la distinción de otro" no existe: lo único que se puede
    ///      repetir es la propia, y eso tiene que rebotar siempre.
    function intentarReacunar(uint256 i) external {
        if (ids.length == 0) return;
        uint256 id = ids[bound(i, 0, ids.length - 1)];
        address de = duenioDe[id];
        uint16 pieza = uint16(id);
        uint64 deadline = uint64(block.timestamp + 1 hours);

        (uint8 v, bytes32 r, bytes32 s) = vm.sign(pkFirmante, _digest(de, pieza, deadline));
        try distinciones.mint(de, pieza, deadline, abi.encodePacked(r, s, v)) {} catch {}
    }

    function _digest(address to, uint16 pieza, uint64 deadline) private view returns (bytes32) {
        (, string memory name, string memory version, uint256 chainId, address verificador,,) =
            distinciones.eip712Domain();

        bytes32 separador = keccak256(
            abi.encode(
                keccak256(
                    "EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"
                ),
                keccak256(bytes(name)),
                keccak256(bytes(version)),
                chainId,
                verificador
            )
        );
        bytes32 hashStruct = keccak256(
            abi.encode(
                keccak256("Distincion(address to,uint16 pieza,uint64 deadline)"),
                to,
                pieza,
                deadline
            )
        );
        return keccak256(abi.encodePacked(hex"1901", separador, hashStruct));
    }
}

/// @notice La promesa del contrato, dicha como invariante: una distinción es de
///         quien la ganó y de nadie más, pase lo que pase.
contract DistincionesInvariantesTest is Base {
    Sacudidor internal sacudidor;

    function setUp() public override {
        super.setUp();

        sacudidor = new Sacudidor(distinciones, PK_FIRMANTE);

        bytes4[] memory selectores = new bytes4[](5);
        selectores[0] = Sacudidor.acunar.selector;
        selectores[1] = Sacudidor.intentarTransferir.selector;
        selectores[2] = Sacudidor.intentarSafeTransferir.selector;
        selectores[3] = Sacudidor.intentarAprobarYTransferir.selector;
        selectores[4] = Sacudidor.intentarReacunar.selector;

        targetContract(address(sacudidor));
        targetSelector(FuzzSelector({addr: address(sacudidor), selectors: selectores}));
    }

    /// @notice Ningún token cambia jamás de dueño.
    function invariant_NingunTokenCambiaDeDuenio() public view {
        uint256 n = sacudidor.cantidadIds();
        for (uint256 k; k < n; ++k) {
            uint256 id = sacudidor.ids(k);
            assertEq(distinciones.ownerOf(id), sacudidor.duenioDe(id), "cambio de duenio");
        }
    }

    /// @notice El suministro es igual a la cantidad de minteos exitosos, contado
    ///         donde se puede contar sin ERC-721Enumerable: billetera por billetera.
    function invariant_ElSuministroEsElDeLosMinteos() public view {
        uint256 total;
        uint256 n = sacudidor.cantidadDuenios();
        for (uint256 k; k < n; ++k) {
            address d = sacudidor.duenios(k);
            assertEq(distinciones.balanceOf(d), sacudidor.tieneQue(d), "saldo distinto");
            total += distinciones.balanceOf(d);
        }
        assertEq(total, sacudidor.acunadas(), "suministro distinto");
    }
}
